package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	pb "github.com/XoRoSh/grpc-server/data"

	"github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func extractFields(selections []ast.Selection) []string {
	var fieldNames []string
	for _, selection := range selections {
		if field, ok := selection.(*ast.Field); ok {
			fieldNames = append(fieldNames, field.Name.Value)
		}
	}
	return fieldNames
}

func createSchema(client pb.DataServiceClient) graphql.Schema {
	// GraphQL dynamically create schema
	dataType := graphql.NewObject(graphql.ObjectConfig{
		Name: "Data",
		Fields: graphql.Fields{
			"id":          &graphql.Field{Type: graphql.String},
			"name":        &graphql.Field{Type: graphql.String},
			"description": &graphql.Field{Type: graphql.String},
		},
	})

	rootQuery := graphql.NewObject(graphql.ObjectConfig{
		Name: "RootQuery",
		Fields: graphql.Fields{
			"getData": &graphql.Field{
				Type: dataType,
				Args: graphql.FieldConfigArgument{
					"id": &graphql.ArgumentConfig{Type: graphql.String},
				},
				Resolve: func(params graphql.ResolveParams) (interface{}, error) {
					id, _ := params.Args["id"].(string)

					// Extract selected fields for the FieldMask
					fieldMask := &fieldmaskpb.FieldMask{
						Paths: extractFields(params.Info.FieldASTs[0].SelectionSet.Selections),
					}

					// Make the grpc call with fieldMask
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

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: rootQuery,
	})
	if err != nil {
		log.Fatalf("Failed to create GraphQL schema: %v", err)
	}
	return schema
}

func main() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()
	client := pb.NewDataServiceClient(conn)

	schema := createSchema(client)

	// GraphQL handler
	http.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
		var params struct {
			Query string `json:"query"`
		}

		if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
			http.Error(w, "Invalid request payload", http.StatusBadRequest)
			return
		}

		result := graphql.Do(graphql.Params{
			Schema:        schema,
			RequestString: params.Query,
		})

		if len(result.Errors) > 0 {
			log.Printf("Failed to execute query: %v", result.Errors)
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(result)
	})

	log.Println("GraphQL server is running on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
