package server

import (
	"fmt"
	"io"
	"log/slog"
	"multiplayer_server/game_map"
	"multiplayer_server/task"
	"multiplayer_server/worker_pool"
	"net"
	"net/http"
	"strconv"
	"strings"
)

func NewServer() *http.ServeMux {
	task.LaunchWorkers(worker_pool.WORKER_COUNT)

	if workerPool := worker_pool.GetWorkerPool(); workerPool.GetAvailableWorkerCount() != worker_pool.WORKER_COUNT {
		panic(fmt.Sprintf("worker pool initialization failed. initialized count: %d, expected count: %d", len(workerPool.Pool), worker_pool.WORKER_COUNT))
	}

	// worker health check
	go task.HealthCheckAndRevive(10)

	game_map.GameMap.Map = &game_map.Map{
		Rows: make([]*game_map.Row, game_map.MAP_SIZE),
	}

	for i := 0; i < int(game_map.MAP_SIZE); i++ {
		game_map.GameMap.Map.Rows[i] = &game_map.Row{
			Cells: make([]*game_map.Cell, game_map.MAP_SIZE),
		}
		for j := 0; j < int(game_map.MAP_SIZE); j++ {
			game_map.GameMap.Map.Rows[i].Cells[j] = &game_map.Cell{
				Kind: game_map.GROUND,
			}
		}
	}

	// coin관련
	game_map.GameMap.InitializeCoins()
	game_map.GameMap.InitializeItems()
	go game_map.GameMap.MoveCoinsRandomly()

	game_map.UserStatuses.UserStatuses = make(map[string]*game_map.UserStatus)
	game_map.GameMap.Scoreboard = make(map[string]int32)

	server := http.NewServeMux()
	server.HandleFunc("GET /get-worker-port/{userId}/{clientPort}", func(w http.ResponseWriter, r *http.Request) {
		userId := r.PathValue("userId")
		// client port는 request에서 얻을 수 없다. 여기서 수령하는 포트는 클라이언트의 UDP 리스닝 포트이기 때문이다.
		clientPort, err := strconv.Atoi(r.PathValue("clientPort"))

		slog.Info("client information", "userId", userId, "clientPort", clientPort)

		w.Header().Set("Content-Type", "text/plain")

		clientIP := net.ParseIP(strings.Split(r.RemoteAddr, ":")[0])

		if clientIP == nil || err != nil || userId == "" {
			w.WriteHeader(http.StatusBadRequest)
			io.WriteString(w, "client information invalid")

			return
		}

		workerPool := worker_pool.GetWorkerPool()
		worker, err := workerPool.Pull()

		if err != nil {
			w.WriteHeader(http.StatusConflict)
			io.WriteString(w, "worker currently not available")

			return
		}

		worker.SetClientInformation(userId, &clientIP, clientPort)

		w.WriteHeader(http.StatusOK)
		io.WriteString(w, fmt.Sprintf("%d", worker.Port))

		worker.StartSendUserRelatedDataToClient()
		game_map.GameMap.Scoreboard[userId] = 0 // 굳이 zero value를 할당하는 이유는 0점이라도 표시가 되어야하기 때문
	})

	server.HandleFunc("PATCH /disconnect/{userId}", func(w http.ResponseWriter, r *http.Request) {
		userId := r.PathValue("userId")

		workerPool := worker_pool.GetWorkerPool()
		workerId, worker, err := workerPool.GetWorkerByUserId(userId)

		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			io.WriteString(w, "worker not found")
			return
		}

		workerPool.Put(workerId, worker)
		delete(game_map.GameMap.Scoreboard, userId)

		w.WriteHeader(http.StatusOK)
		io.WriteString(w, "worker successfully returned to pool")
	})

	// 서버 상태를 조회하기 위한 간단한 핸들러
	server.HandleFunc("GET /server-state", func(w http.ResponseWriter, r *http.Request) {
		workerPool := worker_pool.GetWorkerPool()
		workerCount := workerPool.GetAvailableWorkerCount()
		coinCount := len(game_map.GameMap.Coins)
		itemCount := len(game_map.GameMap.RandomItems)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		io.WriteString(w, fmt.Sprintf(`{"workerCount": %d, "coinCount": %d, "itemCount": %d}`, workerCount, coinCount, itemCount))
	})

	return server
}
