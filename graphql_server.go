package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	pb "../grpc/data"

	"github.com/graphql-go/graphql"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func extractFields(selectionSet *graphql.SelectionSet) []string {
	var fields []string
	for _, selection := range selectionSet.Selections {
		if field, ok := selection.(*graphql.Field); ok {
			fields = append(fields, field.Name.Value)
		}
	}
	return fields
}

func createSchema(client pb.DataServiceClient) graphql.Schema {
	// Define GraphQL types
	dataType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Data",
		Fields: graphql.Fields{
			"id":          &graphql.Field{Type: graphql.String},
			"name":        &graphql.Field{Type: graphql.String},
			"description": &graphql.Field{Type: graphql.String},
		},
	})

	// Define the root query
	rootQuery := graphql.NewObject(graphql.ObjectConfig{
		Name: "RootQuery",
		Fields: graphql.Fields{
			"getData": &graphql.Field{
				Type: dataType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.String},
				},
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					// Extract the `id` argument
					id, _ := params.Args["id"].(string)

					// Extract selected fields for the FieldMask
					fieldMask := &fieldmaskpb.FieldMask{
						Paths: extractFields(&params.Info.FieldASTs[0].SelectionSet),
					}

					// Make the gRPC request with the FieldMask
					res, err := client.GetData(context.Background(), &pb.DataRequest{
						Id:        id,
						FieldMask: fieldMask,
					})
					if err != nil {
						return nil, err
					}

					return res, nil
				},
			},
		},
	})

	// Create schema
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: rootQuery,
	})
	if err != nil {
		log.Fatalf("Failed to create GraphQL schema: %v", err)
	}
	return schema
}

func main() {
	// Connect to gRPC server
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()
	client := pb.NewDataServiceClient(conn)

	// Create GraphQL schema
	schema := createSchema(client)

	// GraphQL handler
	http.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		result := graphql.Do(graphql.Params{
			Schema:        schema,
			RequestString: query,
		})
		if len(result.Errors) > 0 {
			log.Printf("Failed to execute query: %v", result.Errors)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})

	// Start HTTP server
	log.Println("GraphQL server is running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
