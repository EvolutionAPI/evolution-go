package websocket_producer

import (
	"net/http"
	"strings"
	"sync"

	logger_wrapper "github.com/evolution-foundation/evolution-go/pkg/logger"
	"github.com/gomessguii/logger"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		logger.LogInfo("Verificando origem da conexão WebSocket")
		return true
	},
}

// wsConn serializa writes: gorilla/websocket permite apenas um writer concorrente.
type wsConn struct {
	conn    *websocket.Conn
	writeMu sync.Mutex
}

func (w *wsConn) writeJSON(v any) error {
	w.writeMu.Lock()
	defer w.writeMu.Unlock()
	return w.conn.WriteJSON(v)
}

type websocketProducer struct {
	clients       map[string]*wsConn // conexões específicas por instância
	broadcast     []*wsConn          // conexões que recebem todos os eventos
	clientsMux    sync.RWMutex
	loggerWrapper *logger_wrapper.LoggerManager
}

func NewWebsocketProducer(loggerWrapper *logger_wrapper.LoggerManager) *websocketProducer {
	return &websocketProducer{
		clients:       make(map[string]*wsConn),
		broadcast:     make([]*wsConn, 0),
		clientsMux:    sync.RWMutex{},
		loggerWrapper: loggerWrapper,
	}
}

// ServeWs lida com as requisições de upgrade para websocket
func ServeWs(w http.ResponseWriter, r *http.Request, instanceId string, producer *websocketProducer) {
	logger.LogInfo("Iniciando upgrade da conexão WebSocket")
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		logger.LogError("Erro ao fazer upgrade da conexão websocket: %v", err)
		return
	}

	logger.LogInfo("Conexão WebSocket estabelecida com sucesso")

	if instanceId == "" {
		producer.AddBroadcastClient(conn)
	} else {
		producer.AddClient(instanceId, conn)
	}

	// Goroutine para limpar conexão quando fechada
	go func() {
		for {
			_, _, err := conn.ReadMessage()
			if err != nil {
				if instanceId == "" {
					producer.RemoveBroadcastClient(conn)
				} else {
					producer.RemoveClient(instanceId)
				}
				conn.Close()
				break
			}
		}
	}()
}

func (p *websocketProducer) AddBroadcastClient(conn *websocket.Conn) {
	p.clientsMux.Lock()
	defer p.clientsMux.Unlock()
	p.broadcast = append(p.broadcast, &wsConn{conn: conn})
	logger.LogInfo("Cliente broadcast websocket adicionado")
}

func (p *websocketProducer) RemoveBroadcastClient(conn *websocket.Conn) {
	p.clientsMux.Lock()
	defer p.clientsMux.Unlock()
	for i, c := range p.broadcast {
		if c.conn == conn {
			p.broadcast = append(p.broadcast[:i], p.broadcast[i+1:]...)
			break
		}
	}
	logger.LogInfo("Cliente broadcast websocket removido")
}

func (p *websocketProducer) AddClient(instanceID string, conn *websocket.Conn) {
	p.clientsMux.Lock()
	defer p.clientsMux.Unlock()
	p.clients[instanceID] = &wsConn{conn: conn}
	p.loggerWrapper.GetLogger(instanceID).LogInfo("Cliente websocket adicionado para instância: %s", instanceID)
}

func (p *websocketProducer) RemoveClient(instanceID string) {
	p.clientsMux.Lock()
	defer p.clientsMux.Unlock()
	delete(p.clients, instanceID)
	p.loggerWrapper.GetLogger(instanceID).LogInfo("Cliente websocket removido para instância: %s", instanceID)
}

func (p *websocketProducer) Produce(queueName string, payload []byte, instanceID string, _ string) error {
	message := map[string]interface{}{
		"queue":   strings.ToLower(queueName),
		"payload": string(payload),
	}

	p.clientsMux.RLock()
	defer p.clientsMux.RUnlock()

	// Envia para cliente específico da instância
	if client, exists := p.clients[instanceID]; exists {
		err := client.writeJSON(message)
		if err != nil {
			p.loggerWrapper.GetLogger(instanceID).LogError("Erro ao enviar mensagem websocket para %s: %v", instanceID, err)
			// Não remove o cliente aqui pois estamos com o RLock
			return err
		}
		p.loggerWrapper.GetLogger(instanceID).LogInfo("Mensagem websocket enviada com sucesso para instância %s na fila %s", instanceID, queueName)
	}

	// Envia para todos os clientes broadcast
	for _, conn := range p.broadcast {
		err := conn.writeJSON(message)
		if err != nil {
			p.loggerWrapper.GetLogger(instanceID).LogError("Erro ao enviar mensagem broadcast websocket: %v", err)
			continue
		}
	}

	return nil
}

// CreateGlobalQueues não faz nada para websocket producer
func (p *websocketProducer) CreateGlobalQueues() error {
	return nil
}
