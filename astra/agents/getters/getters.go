// astra/agents/getters/getters.go (new)
package getters

import (
	"os/exec"
)

type DataGetters struct {
	fnMaps map[string]func(map[string]interface{}) interface{}
}

func NewDataGetters() *DataGetters {
	g := &DataGetters{fnMaps: make(map[string]func(map[string]interface{}) interface{})}
	g.fnMaps["fetch_codebase_tree"] = g.fetchCodebaseTree
	g.fnMaps["get_db_schema"] = g.getDBSchema
	return g
}

func (g *DataGetters) fetchCodebaseTree(params map[string]interface{}) interface{} {
	path := params["path"].(string)
	cmd := exec.Command("tree", path, "-I", "venv")
	out, err := cmd.Output()
	if err != nil {
		// logging.Logger.Error("fetch_codebase_tree error", "error", err)
		return map[string]interface{}{"error": err.Error()}
	}
	return string(out)
}

func (g *DataGetters) getDBSchema(params map[string]interface{}) interface{} {
	// Use psql to dump schema (integrate with your DAO)
	dbName := params["dbname"].(string)
	cmd := exec.Command("pg_dump", "--schema-only", "-d", dbName)
	out, err := cmd.Output()
	if err != nil {
		return map[string]interface{}{"error": err.Error()}
	}
	return string(out)
}
