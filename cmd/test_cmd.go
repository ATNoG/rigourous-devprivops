package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/Joao-Felisberto/devprivops/database"
	"github.com/Joao-Felisberto/devprivops/fs"
	"github.com/Joao-Felisberto/devprivops/util"
	"github.com/spf13/cobra"
)

// Executes all tests for a given scenario.
// Test failures do not cause an early return, but all other errors do.
//
// `dbManager`: The DBManager connecting to the database
//
// `scenario`: The scenario whose tests are to be executed
//
// returns: true if no test failed, error if there was any error reading files or validating their schemas, connecting to the database or executing queries
func runScenario(dbManager *database.DBManager, scenario database.TestScenario) (bool, error) {
	dbManager.CleanDB()
	slog.Info("Loading scenario", "scenario", scenario.StateDir)

	// 1. Load representations
	err := loadRepresentations(dbManager, scenario.StateDir)
	if err != nil {
		return false, err
	}

	// 2. Load and apply config
	cfgPath := fmt.Sprintf("%s/config.yml", scenario.StateDir)
	// if _, err := os.Stat(cfgPath); !errors.Is(err, os.ErrNotExist) {
	// path/to/whatever does not exist

	err = loadRep(dbManager, cfgPath, "")
	configNotFound := errors.Is(err, os.ErrNotExist)
	if err != nil && !configNotFound {
		return false, err
	}
	if !configNotFound {
		_, err = dbManager.ApplyConfig()
		if err != nil {
			return false, err
		}
	}
	// }
	// 3. Run all the reasoner rules
	if err = reasoner(dbManager); err != nil {
		return false, err
	}

	errors := false

	for _, t := range scenario.Tests {
		slog.Info("Running test", "test", t.Query)
		file, err := fs.GetFile(t.Query)
		if err != nil {
			return false, fmt.Errorf("error reading test file '%s': %s", t.Query, err)
		}
		res, err := dbManager.ExecuteQueryFile(file)
		if err != nil {
			return false, fmt.Errorf("error running test '%s': %s", file, err)
		}

		/*
			a := util.Map(t.ExpectedResult, func(m map[string]interface{}) ComparableJSON { return m })
			b := util.Map(res, func(m map[string]interface{}) ComparableJSON { return m })
			if !util.CompareSets(b, a) {
		*/
		if !util.CompareSets(t.ExpectedResult, res) {
			expected_json, err := json.MarshalIndent(t.ExpectedResult, "", "  ")
			if err != nil {
				return false, fmt.Errorf("could not serialize expected as json: %s", err)
			}
			actual_json, err := json.MarshalIndent(res, "", "  ")
			if err != nil {
				return false, fmt.Errorf("could not serialize actual as json: %s", err)
			}

			errors = true
			fmt.Printf("Expected: %s\n", expected_json)
			fmt.Printf("Actual  : %s\n", actual_json)

			// return fmt.Errorf("result of '%s' does not match expectations: got '%v', expected '%v'", file, res, t.ExpectedResult)
			slog.Error("Test failed", "file", file, "actual", res, "expected", t.ExpectedResult)
		}
	}

	if !errors {
		slog.Info("All tests passed!", "scenario", scenario.StateDir)
	}
	return errors, nil
}

// Main entry point to the test command.
// Run all the tests of each scenario.
//
// `cmd`: The cobra command
//
// `args`: The args of said command
//
// returns: an error when reading any of the scenarios fails
func Test(cmd *cobra.Command, args []string) error {
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

	// 1. Load test metadata
	testFile, err := fs.GetFile("tests/spec.json")
	if err != nil {
		return err
	}

	tests, err := database.TestsFromFile(testFile)
	if err != nil {
		return err
	}

	// 4. For each scenario, run the tests
	errors := false
	for _, t := range tests {
		test_fails, err := runScenario(&dbManager, t)
		if err != nil {
			return fmt.Errorf("test failed for scenario '%s': %s", t.StateDir, err)
		}
		if test_fails {
			errors = true
		}
	}

	if errors {
		return fmt.Errorf("some tests failed")
	}
	return nil
}
