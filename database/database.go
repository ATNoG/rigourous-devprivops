package database

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	attacktree "github.com/Joao-Felisberto/devprivops/attack_tree"
	"github.com/Joao-Felisberto/devprivops/schema"
)

// todo: sanitization https://stackoverflow.com/a/55726984

type QueryMethod string

const (
	QUERY  QueryMethod = "query"
	UPDATE QueryMethod = "update"
	DATA   QueryMethod = "data"
	UPLOAD QueryMethod = "upload"
)

type DBManager struct {
	username string
	password string
	ip       string
	port     int
	dataset  string
}

// var id_cnt = 0

func NewDBManager(
	username string,
	password string,
	ip string,
	port int,
	dataset string,
) DBManager {
	return DBManager{
		username,
		password,
		ip,
		port,
		dataset,
	}
}

func (db *DBManager) sendSparqlQuery(query string, method QueryMethod) (*http.Response, error) {
	endpoint := fmt.Sprintf("http://%s:%d/%s/%s", db.ip, db.port, db.dataset, method)
	client := &http.Client{}

	req, err := http.NewRequest("POST", endpoint, bytes.NewBuffer([]byte(query)))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", fmt.Sprintf("application/sparql-%s", method))
	req.Header.Set("Accept", "application/json")

	auth := db.username + ":" + db.password
	basicAuth := "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
	req.Header.Set("Authorization", basicAuth)

	return client.Do(req)
}

func (db *DBManager) AddTriples(triples []schema.Triple) (int, error) {
	sparqlTemplate := `
		PREFIX ex: <https://example.com/>
        INSERT DATA {
		{{ range . }}{{ .Subject }} {{ .Predicate }} {{ .Object }} .
		{{ end }}
        }
    `
	var sparqlQuery strings.Builder

	tpl := template.Must(template.New("insert triples").Parse(sparqlTemplate))
	if err := tpl.Execute(&sparqlQuery, triples); err != nil {
		return -1, err
	}

	fmt.Printf("Sending %s\n", sparqlQuery.String())

	response, err := db.sendSparqlQuery(sparqlQuery.String(), UPDATE)
	if err != nil {
		fmt.Println("Error sending SPARQL query:", err)
		return -1, err
	}
	defer response.Body.Close()

	return response.StatusCode, nil
}

func (db *DBManager) ExecuteReasonerRule(file string) error {
	sparqlQueryBytes, err := os.ReadFile(file)
	if err != nil {
		return err
	}

	sparqlQuery := string(sparqlQueryBytes)

	fmt.Printf(".Executing %s", sparqlQuery)

	response, err := db.sendSparqlQuery(sparqlQuery, UPDATE)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	return nil
}

func (db *DBManager) ExecuteQueryFile(file string) ([]map[string]interface{}, error) {
	sparqlQueryBytes, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}

	sparqlQuery := string(sparqlQueryBytes)

	response, err := db.sendSparqlQuery(sparqlQuery, QUERY)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	resTxt, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	var resJSON map[string]interface{}
	if err := json.Unmarshal(resTxt, &resJSON); err != nil {
		return nil, err
	}

	results, ok := resJSON["results"].(map[string]interface{})
	if !ok {
		return nil, errors.New("results not found in response")
	}

	bindings, ok := results["bindings"].([]interface{})
	if !ok {
		return nil, errors.New("bindings not found in response")
	}

	var binds []map[string]interface{}
	for _, bind := range bindings {
		bindMap, ok := bind.(map[string]interface{})
		if !ok {
			return nil, errors.New("invalid binding format")
		}
		binds = append(binds, bindMap)
	}

	// fmt.Printf("BEFORE: %d\n", len(binds))
	return binds, nil
}

func (db *DBManager) executeAttackTreeNode(attackNode *attacktree.AttackNode) ([]map[string]interface{}, *attacktree.AttackNode, error) {
	thisNodeIsReachable := len(attackNode.Children) == 0
	for _, node := range attackNode.Children {
		response, failingNode, err := db.executeAttackTreeNode(&node)
		if err != nil {
			return response, failingNode, err
		}
		// fmt.Printf("- %s\n", response)
		if len(response) != 0 {
			thisNodeIsReachable = true
		}
	}
	if thisNodeIsReachable {
		fmt.Printf("Executing %s\n", attackNode.Description)
		binds, qErr := db.ExecuteQueryFile(attackNode.Query)
		// fmt.Printf("AFTER: %d\n", len(binds))
		//		if qErr != nil {
		//			return binds, attackNode, qErr
		//		}
		return binds, attackNode, qErr
	}
	fmt.Printf("HERE\n")
	return nil, nil, nil
}

func (db *DBManager) ExecuteAttackTree(attackTree *attacktree.AttackTree) ([]map[string]interface{}, *attacktree.AttackNode, error) {
	return db.executeAttackTreeNode(&attackTree.Root)
}

func FindQueryFiles(rootDir string) ([]string, error) {
	var files []string

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && filepath.Ext(path) == ".rq" {
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	return files, nil
}
