package test

// 프로그램의 random배치 특성상 유저의 정확한 순위 등을 예측하기는 어렵다.
// 프로젝트의 목적 자체에 가까운 high throughput, low packet size, synchronization등을 테스트한다.

import (
	"encoding/json"
	"fmt"
	"multiplayer_server/game_map"
	"multiplayer_server/server"
	"multiplayer_server/worker_pool"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
)

// 아래의 테스트는 순차적으로 진행되어야 하므로 실패하면 즉시 종료하기 위해 Fatalf로 처리합니다.
var (
	clientListeners []*net.UDPConn
	testServer      *httptest.Server
)

func TestMain(m *testing.M) {
	clientListeners = make([]*net.UDPConn, 0)
	testServer = httptest.NewServer(server.NewServer())

	defer testServer.Close()

	os.Exit(m.Run())
}

func TestInitialResources(t *testing.T) {
	t.Log(testServer)
	resp, err := http.Get(testServer.URL + "/server-state")

	if err != nil {
		t.Fatalf("failed to get server state: %s", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status code: %d, got: %d", http.StatusOK, resp.StatusCode)
	}

	serverState := make(map[string]interface{})
	// json으로 받은 response를 map으로 변환

	json.NewDecoder(resp.Body).Decode(&serverState)

	workerCount := int(serverState["workerCount"].(float64))
	coinCount := int(serverState["coinCount"].(float64))
	itemCount := int(serverState["itemCount"].(float64))
	t.Run("워커 생성(프로그램이 켜질 때 함께 생성됨. 워커 갯수 검사)", func(t *testing.T) {

		if workerCount != worker_pool.WORKER_COUNT {
			t.Fatalf("worker pool initialization failed. expected: %d, go: %d", worker_pool.WORKER_COUNT, workerCount)
		}
	})
	// 기본 자원(map, coin, item) 생성 및 자원 갯수 검사
	t.Run("기본 자원 생성(맵, 코인, 아이템)", func(t *testing.T) {
		// 코인 검사(자원이 맵에 뿌려졌다는 것 자체가 맵이 잘 생성되었다는 것)
		if coinCount > game_map.COIN_COUNT || coinCount == 0 { // <= 조건인 이유는 코인은 랜덤성을 위하여 중복된 위치가 선정될 경우 그냥 스킵해버리기 때문에 COIN_COUNT보다 적게 생성될 수도 있다.
			t.Fatalf("coin count is not correct. expected: %d, got: %d", game_map.COIN_COUNT, len(game_map.GameMap.Coins))
		}

		// 아이템 검사
		if itemCount != game_map.ITEM_COUNT { // 코인과 다르게 아이템은 무조건 ITEM_COUNT만큼 생성되어야 한다.
			t.Fatalf("item count is not correct. expected: %d, got: %d", game_map.ITEM_COUNT, len(game_map.GameMap.RandomItems))
		}
	})
}

func TestWorkerPullOut(t *testing.T) {
	// 최대 수의 유저 로그인(워커풀이 비었음을 검사하고, 추가로 로그인 시도 시 실패)
	t.Run("최대 수의 유저 로그인", func(t *testing.T) {
		// 최대 수의 유저 로그인
		for i := 0; i < worker_pool.WORKER_COUNT; i++ {
			// 로그인 시도
			// client는 UDP로 데이터를 전달 받기 때문에 먼저 UDP connection을 생성해야 한다.
			conn, err := net.ListenUDP("udp", &net.UDPAddr{
				IP:   net.IPv4(127, 0, 0, 1),
				Port: 0, // OS에게 빈 포트 요청
			})

			if err != nil {
				t.Fatalf("failed to create UDP connection: %s", err)
			}

			clientListeners = append(clientListeners, conn)
			clientPort := conn.LocalAddr().(*net.UDPAddr).Port
			// 로그인 성공 검사
			// 유저 아이디 생성
			userId := "user" + strconv.Itoa(i)
			http.Get(fmt.Sprintf(testServer.URL+"/get-worker-port/%s/%d", userId, clientPort))
		}
		// 최대 유저를 넘어선 로그인 시도
		// 로그인 실패 검사
		userId := "user-over-limit"
		resp, _ := http.Get(fmt.Sprintf(testServer.URL+"/get-worker-port/%s/%d", userId, 0))

		if resp.StatusCode != http.StatusConflict {
			t.Fatalf("expected status code: %d, got: %d", http.StatusConflict, resp.StatusCode)
		}
	})
}

func TestPlay(t *testing.T) {
	// 유저의 랜덤 위치 이동(스코어가 쌓일 시간을 주기 위해 10초 실행. 복수의 유저가 크래시 없이 game map과 scoreboard에 write하고 점수를 쌓는 것 자체가 테스트의 목적)
	t.Run("유저의 랜덤 위치 이동", func(t *testing.T) {

	})
}

func TestWorkerPutBack(t *testing.T) {
	t.Run("유저 disconnect시 worker 반환", func(t *testing.T) {})
}

// 워커에 이상이 생겼을 때(health check에 실패했을 때) 복구 되는지 검사