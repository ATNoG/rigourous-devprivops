// Package for each of the commands' internal logic
package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	attacktree "github.com/Joao-Felisberto/devprivops/attack_tree"
	"github.com/Joao-Felisberto/devprivops/database"
	"github.com/Joao-Felisberto/devprivops/fs"
	"github.com/Joao-Felisberto/devprivops/schema"
	"github.com/Joao-Felisberto/devprivops/util"
	"github.com/spf13/cobra"
)

// Run all reasoner rules
//
// # The reasoner rules live under the `reasoner` subdirectory under each configuration directory
//
// `dbManager`: The DBManager connecting to the database
//
// returns: an error when the rule could not be read from the file or run
func reasoner(dbManager *database.DBManager) error {
	slog.Info("===Reasoner Rules===")
	reasonDir, err := fs.GetFile("reasoner")
	if err != nil {
		return err
	}
	files, err := os.ReadDir(reasonDir)
	if err != nil {
		return err
	}
	for _, file := range files {
		fPath, err := fs.GetFile(fmt.Sprintf("reasoner/%s", file.Name()))
		if err != nil {
			return err
		}
		if err := dbManager.ExecuteReasonerRule(fPath); err != nil {
			return fmt.Errorf("could not execute reasoner rule: %s", err)
		}
	}

	return nil
}

// Runs all policies of a regulation
//
// `dbManager`: The DBManager connecting to the database
// `regulation`: The path to the regulation (relative to the regulations path)
//
// returns: the execution report if everything succeeds, or an error when the policy could not be read from the file, does not abide by the schema, or has execution errors
func policies(dbManager *database.DBManager, regulation string) ([]map[string]interface{}, error) {
	slog.Info("===Policy Compliance===")
	polFile, err := fs.GetFile(fmt.Sprintf("regulations/%s/policies.yml", regulation))
	if err != nil {
		return nil, err
	}
	polSchema, err := fs.GetFile("schemas/query-schema.json")
	if err != nil {
		return nil, err
	}
	yamlQueries, err := schema.ReadYAML(polFile, polSchema)
	if err != nil {
		return nil, err
	}

	yamlQueriesList := yamlQueries.([]interface{})
	queries := util.Map(yamlQueriesList, func(q1 interface{}) database.Query {
		q := q1.(map[interface{}]interface{})
		// format := q["format"].(map[interface{}]interface{})

		qFile, err := fs.GetFile(q["file"].(string))
		if err != nil {
			// very beautiful, isn't it?
			panic(err)
		}
		// maxViolations, err := strconv.Atoi(q["maximum violations"].(string))
		maxViolations := q["maximum violations"].(int)
		if err != nil {
			// will never happen because the schema has already been validated
			panic(err)
		}
		return database.NewQuery(
			// fmt.Sprintf("./.%s/%s", appName, q["file"].(string)),
			qFile,
			q["title"].(string),
			q["description"].(string),
			q["is consistency"].(bool),
			maxViolations,
			q["mapping message"].(string),
		)
	})
	report := []map[string]interface{}{}
	for _, pol := range queries {
		res, err := dbManager.ExecuteQueryFile(pol.File)
		if err != nil {
			return nil, fmt.Errorf("error executing query from '%s': %s", pol.File, err)
		}
		// TODO: operate on the results
		b, err := json.MarshalIndent(res, "", "  ")
		if err != nil {
			slog.Error("error parsing query results:", "error", err)
		}
		slog.Info("Violations:", "policy", pol.Title, "violations", b)
		report = append(report, map[string]interface{}{
			"name":               pol.Title,
			"description":        pol.Description,
			"maximum violations": pol.MaxViolations,
			"is consistency":     pol.IsConsistency,
			"violations":         res,
			"mapping message":    pol.MappingMessage,
		})
	}

	return report, nil
}

// Execute all the attack/harm trees
//
// `dbManager`: The DBManager connecting to the database
//
// returns: the execution report if everything succeeds, or an error when the tree could not be read from the file or does not abide by the schema
func attackTrees(dbManager *database.DBManager) ([]*attacktree.AttackTree, error) {
	slog.Info("===Attack Trees===")
	atkDir, err := fs.GetFile("attack_trees/descriptions/")
	if err != nil {
		return nil, err
	}
	files, err := os.ReadDir(atkDir)
	if err != nil {
		return nil, err
	}
	atkSchema, err := fs.GetFile("schemas/atk-tree-schema.json")
	if err != nil {
		return nil, err
	}
	report := []*attacktree.AttackTree{}
	for _, file := range files {
		fPath, err := fs.GetFile(fmt.Sprintf("attack_trees/descriptions/%s", file.Name()))
		if err != nil {
			return nil, err
		}
		tree, err := attacktree.NewAttackTreeFromYaml(fPath, atkSchema)
		if err != nil {
			return nil, err
		}

		// query code, failingNode, err
		_, failingNode, err := dbManager.ExecuteAttackTree(tree)
		if err != nil {
			return nil, fmt.Errorf("error at node '%s': %s", failingNode.Description, err)
		}

		report = append(report, tree)
	}
	return report, nil
}

// Takes the report and validates whether the system has only acceptable flaws and can pass to the next steps of the pipeline
//
// `report`: the final report
//
// returns: the list of unacceptable violations
func validateReport(report *map[string]interface{}) []string {
	regulations := (*report)["policies"].([]interface{})
	violated := []string{}

	/*
		m, err := json.MarshalIndent(regulations, "", "  ")
		if err != nil {
			return []string{}
		}
		fmt.Println(string(m))
	*/

	for _, regulation := range regulations {
		policies := (regulation.(map[string]interface{}))["results"]
		for _, policy := range policies.([]map[string]interface{}) {
			// policy = policy.(map[string]interface{})
			maxViolations := policy["maximum violations"].(int)
			violations := len(policy["violations"].([]map[string]interface{}))

			if violations > maxViolations {
				violated = append(violated, policy["name"].(string))
			}
		}
	}

	return violated
}

// Runs the requirements queries to check whether or not the system supports the implementation of the requirements
//
// `dbManager`: The DBManager connecting to the database
//
// returns: the execution report if everything succeeds, or an error when the requirements could not be read from the file or does not abide by the schema, or the execution of a requirement was not cmopleted successfully
func verifyRequirements(dbManager *database.DBManager) (*[]map[string]interface{}, error) {
	requirementsFile, err := fs.GetFile("requirements/requirements.yml")
	if err != nil {
		return nil, err
	}
	requirementsSchema, err := fs.GetFile("schemas/requirement-schema.json")
	if err != nil {
		return nil, err
	}
	requirementsRaw, err := schema.ReadYAML(requirementsFile, requirementsSchema)
	if err != nil {
		return nil, err
	}
	userStories, err := database.USFromYAML(requirementsRaw.([]interface{}))
	if err != nil {
		return nil, err
	}

	report := []map[string]interface{}{}
	for _, us := range userStories {
		usReport := map[string]interface{}{
			"use case":       us.UseCase,
			"is misuse case": us.IsMisuseCase,
			"requirements":   []map[string]interface{}{},
		}
		for _, r := range us.Requirements {
			f, err := fs.GetFile(r.Query)
			if err != nil {
				return nil, err
			}
			res, err := dbManager.ExecuteQueryFile(f)
			if err != nil {
				return nil, err
			}
			if res == nil {
				res = []map[string]interface{}{}
			}
			// usReport := report[us.UseCase].(map[string]interface{})
			usReport["requirements"] = append(usReport["requirements"].([]map[string]interface{}),
				map[string]interface{}{
					"title":       r.Title,
					"description": r.Description,
					"results":     res,
				},
			)
		}
		report = append(report, usReport)
	}

	return &report, nil
}

// Runs the queries for extra data to be included in the report
//
// `dbManager`: The DBManager connecting to the database
//
// returns: the execution report if everything succeeds, or an error when the queries could not be read from the file or does not abide by the schema, or the execution of a query was not completed successfully
func getExtraData(dbManager *database.DBManager) (*[]map[string]interface{}, error) {
	slog.Info("===Extra Data===")

	extraDataFile, err := fs.GetFile("report_data/report_data.yml")
	if err != nil {
		return nil, err
	}
	extraDataSchema, err := fs.GetFile("schemas/report_data-schema.json")
	if err != nil {
		return nil, err
	}
	extraDataRaw, err := schema.ReadYAML(extraDataFile, extraDataSchema)
	if err != nil {
		return nil, err
	}

	extraData := extraDataRaw.([]interface{})
	report := util.Map(extraData, func(dRaw interface{}) map[string]interface{} {
		d := util.MapCast[string, interface{}](dRaw.(map[interface{}]interface{}))

		f, err := fs.GetFile(d["query"].(string))
		if err != nil {
			panic(fmt.Sprintf("Error getting query file %s", d["query"].(string)))
		}

		slog.Info("Getting extra information:", "query", f)
		d["results"], err = dbManager.ExecuteQueryFile(f)
		if err != nil {
			panic(fmt.Sprintf("Error processing query: %s", err))
		}
		resJson, err := json.Marshal(d["results"])
		if err != nil {
			panic(fmt.Sprintf("Error marshaling the results: %s", err))
		}
		slog.Info("Extra information extracted:", "info", resJson)

		delete(d, "query")
		return d
	})

	return &report, nil
}

// Sends the provided report through HTTP to a server  that can read it
//
// `url`: The server URL
//
// `report`: The report to send
//
// returns: an error if there are issues sending the report
func sendReport(url string, report *map[string]interface{}) error {
	// Read report.json file
	reportData, err := os.ReadFile("report.json")
	if err != nil {
		return fmt.Errorf("error reading report.json: %s", err)
	}

	// Send HTTP POST request
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(reportData))
	if err != nil {
		return fmt.Errorf("error sending HTTP request: %s", err)
	}
	defer resp.Body.Close()

	return nil
}

// Main entry point for the `analyse` command
// Analyse the system descriptions to know whether the system abides by the provided rules
//
// The execution flow is as follows:
//  1. Load DFD into DB
//  2. Run all the reasoner rules
//  3. Verify policy compliance
//  4. Run all attack trees
//  7. Check whether the violations are acceptable
//  8. Validate whether requirements are met
//  9. Get extra data
//  10. Send the report to the site
//
// `cmd`: The cobra command
//
// `args`: The args of said command
//
// returns: an error when any of the phases fails
func Analyse(cmd *cobra.Command, args []string) error {
	username := args[0]
	password := args[1]
	ip := args[2]
	port, err := strconv.Atoi(args[3])
	if err != nil {
		return err
	}
	dataset := args[4]
	reportEndpoint := cmd.Flag("report-endpoint").Value.String()

	dbManager := database.NewDBManager(
		username,
		password,
		ip,
		port,
		dataset,
	)
	dbManager.CleanDB()

	report := map[string]interface{}{}

	// 1. Load DFD into DB
	if err = loadRepresentations(&dbManager, "descriptions"); err != nil {
		return err
	}

	// 2. Load and apply config
	err = loadRep(&dbManager, "config/config.yml", "")
	if err != nil {
		return err
	}
	_, err = dbManager.ApplyConfig()
	if err != nil {
		return err
	}

	// 2. Run all the reasoner rules
	if err = reasoner(&dbManager); err != nil {
		return err
	}

	// 3. Verify policy compliance
	report["policies"] = []interface{}{}
	regulations, err := fs.GetRegulations()
	if err != nil {
		return err
	}
	for _, regulation := range regulations {
		// reg := report["policies"].([]interface{})
		polReport, err := policies(&dbManager, regulation)
		if err != nil {
			return err
		}
		report["policies"] = append(report["policies"].([]interface{}), map[string]interface{}{
			"name":    regulation,
			"results": polReport,
		})
		// reg[regulation] = polReport
	}

	// 4. Run all attack trees
	atkReport, err := attackTrees(&dbManager)
	if err != nil {
		return err
	}
	report["attack trees"] = atkReport

	// 5. Clean database
	// dbManager.CleanDB()

	// 6. Print and store report
	//	gitCommit := exec.Command("git", "rev-parse", "HEAD")
	//	var commitOut bytes.Buffer
	//	gitCommit.Stdout = &commitOut

	gitBranch := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	var branchOut bytes.Buffer
	gitBranch.Stdout = &branchOut

	//	gitCommit.Run()
	gitBranch.Run()

	// time := fmt.Sprint(time.Now().Unix())
	time := time.Now().Unix()

	projDir, err := os.Getwd()
	if err != nil {
		return err
	}
	projPath := strings.Split(projDir, "/")
	projDir = projPath[len(projPath)-1]

	report["branch"] = strings.Trim(branchOut.String(), "\n")
	// report["time"] = commitOut.String()
	report["time"] = time
	report["project"] = projDir

	// jsonReport, err := json.MarshalIndent(report, "", "  ")

	// 7. Check whether the violations are acceptable
	violations := validateReport(&report)
	if len(violations) != 0 {
		slog.Error("There are policies with too many violations\n")
		for _, v := range violations {
			slog.Error(fmt.Sprintf("\t- %s\n", v))
		}
		// os.Exit(1)
	}

	// 8. Validate whether requirements are met
	usReport, err := verifyRequirements(&dbManager)
	if err != nil {
		slog.Error("Error validating requirements", "error", err)
	}
	report["user stories"] = usReport

	// 9. Get extra data
	extraData, err := getExtraData(&dbManager)
	if err != nil {
		slog.Error("Error fetching extra report data", "error", err)
	}
	report["extra data"] = extraData

	// 10. Send the report to the site
	jsonReport, err := json.Marshal(report)
	if err != nil {
		slog.Error("error parsing report:", "error", err)
	}

	slog.Info("Report written to 'report.json'")
	if err := os.WriteFile("report.json", []byte(jsonReport), 0666); err != nil {
		return err
	}
	if reportEndpoint != "" {
		if err := sendReport(reportEndpoint, &report); err != nil {
			return err
		}
	}

	return nil
}