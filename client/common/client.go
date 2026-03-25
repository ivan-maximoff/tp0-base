package common

import (
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/op/go-logging"
)

var log = logging.MustGetLogger("log")

// ClientConfig Configuration used by the client
type ClientConfig struct {
	ID            string
	ServerAddress string
	LoopAmount    int
	LoopPeriod    time.Duration
}

// Client Entity that encapsulates how
type Client struct {
	config ClientConfig
	conn   net.Conn
	stop   chan os.Signal
}

// NewClient Initializes a new client receiving the configuration
// as a parameter
func NewClient(config ClientConfig) *Client {
	client := &Client{
		config: config,
		stop:   make(chan os.Signal, 1),
	}
	signal.Notify(client.stop, syscall.SIGTERM)
	return client
}

// CreateClientSocket Initializes client socket. In case of
// failure, error is printed in stdout/stderr and exit 1
// is returned
func (c *Client) createClientSocket() error {
	conn, err := net.Dial("tcp", c.config.ServerAddress)
	if err != nil {
		log.Criticalf(
			"action: connect | result: fail | client_id: %v | error: %v",
			c.config.ID,
			err,
		)
		return err
	}
	c.conn = conn
	return nil
}

// StartClientLoop Send messages to the client until some time threshold is met
// or a termination signal (SIGTERM) is received.
func (c *Client) StartClientLoop() {
	bet := Bet{
        Agency:    os.Getenv("CLI_ID"),
        Name:      os.Getenv("NOMBRE"),
        LastName:  os.Getenv("APELLIDO"),
        ID:        os.Getenv("DOCUMENTO"),
        BirthDate: os.Getenv("NACIMIENTO"),
        Number:    os.Getenv("NUMERO"),
    }
	for msgID := 1; msgID <= c.config.LoopAmount; msgID++ {
		// Check for termination signal before starting a new connection
		select {
		case <-c.stop:
			log.Infof("action: signal_received | result: in_progress | signal: SIGTERM")
			if c.conn != nil {
				c.conn.Close()
				log.Infof("action: close_client_socket | result: success")
			}
			log.Infof("action: client_shutdown | result: success")
			return
		default:
		}

		// Create the connection the server in every loop iteration. Send an
		err := c.createClientSocket()
        if err != nil {
            log.Errorf("action: connect | result: fail | error: %v", err)
            time.Sleep(c.config.LoopPeriod)
            continue
        }

		payload := bet.Serialize()
		if err := WriteFrame(c.conn, OpcodeBet, payload); err != nil {
            log.Errorf("action: send_bet | result: fail | error: %v", err)
            c.conn.Close()
            return
        }

		frame, err := ReadFrame(c.conn)
        if err != nil {
            log.Errorf("action: receive_ack | result: fail | error: %v", err)
        } else if frame.Opcode == OpcodeAck {
            log.Infof("action: apuesta_enviada | result: success | dni: %s | numero: %s", bet.ID, bet.Number)
        } else {
            log.Errorf("action: receive_ack | result: fail | error: received_opcode_%v", frame.Opcode)
        }

        c.conn.Close()

		// Wait between messages or interrupt immediately if a signal arrives
		select {
		case <-time.After(c.config.LoopPeriod):
			// Period reached, continue to next iteration
		case <-c.stop:
			log.Infof("action: signal_received | result: in_progress | signal: SIGTERM")
			log.Infof("action: client_shutdown | result: success")
			return
		}
	}
	log.Infof("action: loop_finished | result: success | client_id: %v", c.config.ID)
}
