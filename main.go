package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	attacktree "github.com/Joao-Felisberto/devprivops/attack_tree"
	"github.com/Joao-Felisberto/devprivops/database"
	"github.com/Joao-Felisberto/devprivops/schema"
	"github.com/Joao-Felisberto/devprivops/util"
	"github.com/spf13/cobra"
)

func main() {
	appName := "devprivops"

	dfdSchema := fmt.Sprintf("./.%s/schemas/dfd-schema.json", appName)
	attackTreeSchema := fmt.Sprintf("./.%s/schemas/atk-tree-schema.json", appName)

	var rootCmd = &cobra.Command{
		Use:   appName,
		Short: fmt.Sprintf("A CLI application to analyze %s", appName),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("please specify a subcommand. Use '%s --help' for usage details", appName)
		},
	}

	var analyseCmd = &cobra.Command{
		Use:   "analyse <username> <password> <database ip> <database port> <dataset>",
		Short: fmt.Sprintf("Analyse the specified database endpoint for %s", appName),
		Args:  cobra.ExactArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			username := args[0]
			password := args[1]
			ip := args[2]
			port, err := strconv.Atoi(args[3])
			if err != nil {
				return err
			}
			dataset := args[4]

			dbManager := database.NewDBManager(
				username,
				password,
				ip,
				port,
				dataset,
			)

			// 1. Load DFD into DB
			fmt.Println("===\nLoading DFD into DB\n===")
			dfd, err := schema.ReadYAML(
				fmt.Sprintf("./.%s/dfd/dfd.yml", appName),
				fmt.Sprintf("./.%s/dfd/dfd-schema.json", appName),
			)
			if err != nil {
				return err
			}
			statusCode, err := dbManager.AddTriples(schema.YAMLtoRDF("https://example.com/ROOT", dfd, "https://example.com/ROOT"))
			if err != nil {
				return err
			}
			if statusCode != 204 {
				return fmt.Errorf("unexpected status code: %d", statusCode)
			}

			// 2. Run all the reasoner rules
			fmt.Println("===\nReasoner Rules\n===")
			reasonDir := fmt.Sprintf("./.%s/reasoner/", appName)
			files, err := os.ReadDir(reasonDir)
			if err != nil {
				return err
			}
			for _, file := range files {
				fPath := fmt.Sprintf("./.%s/reasoner/%s", appName, file.Name())
				if err := dbManager.ExecuteReasonerRule(fPath); err != nil {
					return fmt.Errorf("could not execute reasoner rule: %s", err)
				}
			}

			// 3. Verify policy compliance
			fmt.Println("===\nPolicy Compliance\n===")
			/*
				polDir := fmt.Sprintf("./.%s/policies/", appName)
				polFiles, err := database.FindQueryFiles(polDir)
				if err != nil {
					return err
				}
				for _, pol := range polFiles {
					res, err := dbManager.ExecuteQueryFile(pol)
					if err != nil {
						return err
					}
					// TODO: operate on the results
					fmt.Printf("%s\n", res)
				}
			*/
			polFile := fmt.Sprintf("./.%s/policies/policies.yml", appName)
			polschema := fmt.Sprintf("./.%s/policies/query-schema.json", appName)
			yamlQueries, err := schema.ReadYAML(polFile, polschema)
			if err != nil {
				return err
			}
			// fmt.Printf("MAIN: %s\n", yamlQueries)
			// yamlQueriesList := yamlQueries.([]map[string]interface{})
			// queries := util.Map(yamlQueriesList, func(q map[string]interface{}) database.Query {
			yamlQueriesList := yamlQueries.([]interface{})
			queries := util.Map(yamlQueriesList, func(q1 interface{}) database.Query {
				q := q1.(map[interface{}]interface{})
				format := q["format"].(map[interface{}]interface{})
				return database.NewQuery(
					fmt.Sprintf("./.%s/%s", appName, q["file"].(string)),
					q["title"].(string),
					q["description"].(string),
					format["heading whith results"].(string),
					format["heading without results"].(string),
					format["result line"].(string),
				)
			})
			for _, pol := range queries {
				res, err := dbManager.ExecuteQueryFile(pol.File)
				if err != nil {
					return fmt.Errorf("error executing query from '%s': %s", pol.File, err)
				}
				// TODO: operate on the results
				b, err := json.MarshalIndent(res, "", "  ")
				if err != nil {
					fmt.Println("error parsing query results:", err)
				}
				fmt.Printf("Violations of '%s': %s\n", pol.Title, b)
			}

			// 4. Verify contract compliance
			/*
				contractDir := fmt.Sprintf("./.%s/contracts/", appName)
				contractFiles, err := database.FindQueryFiles(contractDir)
				if err != nil {
					return err
				}
				for _, con := range contractFiles {
					res, err := dbManager.ExecuteQueryFile(con)
					if err != nil {
						return err
					}
					// TODO: operate on the results
					fmt.Printf("%s\n", res)
				}
			*/
			/* // TODO uncomment
			fmt.Println("===\nContract Compliance\n===")
			contractFile := fmt.Sprintf("./.%s/contracts/contracts.yml", appName)
			yamlQueries, err = schema.ReadYAML(contractFile, polschema)
			if err != nil {
				return err
			}
			// yamlQueriesList = yamlQueries.([]map[string]interface{})
			// queries = util.Map(yamlQueriesList, func(q map[string]interface{}) database.Query {
			yamlQueriesList = yamlQueries.([]interface{})
			queries = util.Map(yamlQueriesList, func(q1 interface{}) database.Query {
				q := q1.(map[interface{}]interface{})
				format := q["format"].(map[interface{}]interface{})
				return database.NewQuery(
					// q["file"].(string),
					fmt.Sprintf("./.%s/%s", appName, q["file"].(string)),
					q["title"].(string),
					q["description"].(string),
					format["heading whith results"].(string),
					format["heading without results"].(string),
					format["result line"].(string),
				)
			})
			for _, contract := range queries {
				res, err := dbManager.ExecuteQueryFile(contract.File)
				if err != nil {
					return err
				}
				// TODO: operate on the results
				fmt.Printf("%s\n", res)
			}
			*/
			// 5. Run all attack trees
			fmt.Println("===\nAttack Trees\n===")
			atkDir := fmt.Sprintf("./.%s/attack_trees/", appName)
			files, err = os.ReadDir(atkDir)
			if err != nil {
				return err
			}
			atkSchema := fmt.Sprintf("./.%s/atk-tree-schema.json", appName)
			for _, file := range files {
				fPath := fmt.Sprintf("./.%s/attack_trees/%s", appName, file.Name())
				tree, err := attacktree.NewAttackTreeFromYaml(fPath, atkSchema)
				if err != nil {
					// fmt.Printf("ERROR!!!! %s: %s\n", fPath, err)
					return err
				}

				// query code, failingNode, err
				_, failingNode, err := dbManager.ExecuteAttackTree(tree)
				if err != nil {
					return fmt.Errorf("error at node '%s': %s", failingNode.Description, err)
				}
			}

			// fmt.Printf("Analyzing database at endpoint: %s:%d\n", ip, port)
			return nil
		},
	}

	var devCmd = &cobra.Command{
		Use:   "dev",
		Short: "Development tests only",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Running development command")
			return nil
		},
	}

	analyseCmd.Flags().StringVar(&dfdSchema, "dfd-schema", dfdSchema, "Custom DFD schema file")
	analyseCmd.Flags().StringVar(&attackTreeSchema, "attack-tree-schema", attackTreeSchema, "Custom attack tree schema file")

	rootCmd.AddCommand(analyseCmd)
	rootCmd.AddCommand(devCmd)

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
