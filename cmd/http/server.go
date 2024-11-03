package main

import (
	"graphql-hello/db"
	"log"
	"net/http"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/handler"
	"github.com/rs/cors"
)

type Pokemon struct {
	ID         int64     `db:"id" json:"id"`
	Name       string    `db:"name" json:"name"`
	DexID      int       `db:"dex_id" json:"dexId"`
	UpVotes    int       `db:"up_votes" json:"upVotes"`
	DownVotes  int       `db:"down_votes" json:"downVotes"`
	InsertedAt time.Time `db:"inserted_at" json:"insertedAt"`
	UpdatedAt  time.Time `db:"updated_at" json:"updatedAt"`
}

func main() {
	// Connect to database
	db, err := db.NewConnection()
	if err != nil {
		log.Fatalf("Could not connect to database: %v", err)
	}
	defer db.Close()

	// Define Character type
	pokemonType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Pokemon",
		Fields: graphql.Fields{
			"id":        &graphql.Field{Type: graphql.Int},
			"name":      &graphql.Field{Type: graphql.String},
			"dexId":     &graphql.Field{Type: graphql.Int},
			"upVotes":   &graphql.Field{Type: graphql.Int},
			"downVotes": &graphql.Field{Type: graphql.Int},
		},
	})

	// Definite mutations
	mutations := graphql.Fields{
		"vote": &graphql.Field{
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "VoteResult",
				Fields: graphql.Fields{
					"success": &graphql.Field{Type: graphql.Boolean},
				},
			}),
			Args: graphql.FieldConfigArgument{
				"upvoteId": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.Int),
				},
				"downvoteId": &graphql.ArgumentConfig{
					Type: graphql.NewNonNull(graphql.Int),
				},
			},
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				upvoteId := p.Args["upvoteId"].(int)
				downvoteId := p.Args["downvoteId"].(int)

				tx, err := db.Begin()
				if err != nil {
					return nil, err
				}
				defer tx.Rollback()

				// Fixed the query to properly use a single parameter
				_, err = tx.Exec(`
                    UPDATE pokemon 
					SET up_votes = up_votes + 1
                    WHERE id = $1;
                `, upvoteId)
				if err != nil {
					return nil, err
				}

				// Fixed the query to properly use a single parameter
				_, err = tx.Exec(`
                    UPDATE pokemon 
					SET down_votes = down_votes + 1
                    WHERE id = $1;
                `, downvoteId)
				if err != nil {
					return nil, err
				}

				err = tx.Commit()
				if err != nil {
					return nil, err
				}

				return map[string]interface{}{
					"success": true,
				}, nil
			},
		},
	}

	// Define query
	fields := graphql.Fields{
		"pokemons": &graphql.Field{
			Type: graphql.NewList(pokemonType),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				var pokemons []Pokemon
				err := db.Select(&pokemons, "SELECT name, id, up_votes, down_votes, inserted_at, updated_at FROM pokemon ORDER BY up_votes DESC")
				if err != nil {
					return nil, err
				}
				return pokemons, nil
			},
		},
		"randomPair": &graphql.Field{
			Type: graphql.NewObject(graphql.ObjectConfig{
				Name: "RandomPair",
				Fields: graphql.Fields{
					"pokemonOne": &graphql.Field{Type: pokemonType},
					"pokemonTwo": &graphql.Field{Type: pokemonType},
				},
			}),
			Resolve: func(p graphql.ResolveParams) (interface{}, error) {
				var pokemons []Pokemon

				err := db.Select(&pokemons, `
				SELECT name, id, dex_id, up_votes, down_votes, inserted_at, updated_at FROM pokemon
                    ORDER BY RANDOM() 
                    LIMIT 2
                `)
				if err != nil {
					return nil, err
				}

				if len(pokemons) < 2 {
					return nil, nil
				}

				return map[string]interface{}{
					"pokemonOne": pokemons[0],
					"pokemonTwo": pokemons[1],
				}, nil
			},
		},
	}

	rootQuery := graphql.ObjectConfig{
		Name:   "RootQuery",
		Fields: fields,
	}

	schemaConfig := graphql.SchemaConfig{
		Query: graphql.NewObject(rootQuery),
		Mutation: graphql.NewObject(graphql.ObjectConfig{
			Name:   "Mutation",
			Fields: mutations,
		}),
	}

	schema, err := graphql.NewSchema(schemaConfig)
	if err != nil {
		log.Fatalf("Failed to create schema: %v", err)
	}

	// Create handler
	h := handler.New(&handler.Config{
		Schema:   &schema,
		Pretty:   true,
		GraphiQL: true,
	})

	// Setup CORS
	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Authorization", "Content-Type"},
		Debug:          true,
	})

	// Apply CORS middleware
	handler := c.Handler(h)

	// Start server
	http.Handle("/graphql", handler)
	log.Println("Server is running on http://localhost:8080/graphql")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
