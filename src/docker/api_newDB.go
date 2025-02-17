package docker

import (
	"net/http"
	"encoding/json"
	"time"
	"os"

	"github.com/azukaar/cosmos-server/src/utils" 
)

func restart() {
	time.Sleep(3 * time.Second)
	os.Exit(0)
}

func NewDBRoute(w http.ResponseWriter, req *http.Request) {
	if utils.AdminOnly(w, req) != nil {
		return
	}

	if(req.Method == "GET") {
		costr, err := NewDB()

		if err != nil {
			utils.Error("NewDB: Error while creating new DB", err)
			utils.HTTPError(w, "Error while creating new DB", http.StatusInternalServerError, "DB001")
			return
		}

		config := utils.GetBaseMainConfig()
		config.MongoDB = costr
		utils.SaveConfigTofile(config)
		
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "OK",
		})

		go restart()
	} else {
		utils.Error("UserList: Method not allowed" + req.Method, nil)
		utils.HTTPError(w, "Method not allowed", http.StatusMethodNotAllowed, "HTTP001")
		return
	}
}