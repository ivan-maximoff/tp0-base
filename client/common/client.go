package common

import (
	"encoding/csv"
	"fmt"
	"io"
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
	MaxAmount     int
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

// StartClientLoop reads the dataset and sends bets in batches to the server.
// It respects both maximum batch size in bytes and maximum number of bets per batch.
func (c *Client) StartClientLoop() {
	file, reader, err := c.openDataset()
    if err != nil {
        log.Errorf("action: open_dataset | result: fail | error: %v", err)
        return
    }
    defer file.Close()

	const maxBatchBytes = 8192
	batch := make([]Bet, 0, c.config.MaxAmount)
	currentBatchBytes := 0

	for {
		if c.isStopped() {
			c.handleShutdown()
			return
		}

		record, err := reader.Read()
        if err == io.EOF {
            break // End of file reached
        }
		if err != nil {
            log.Errorf("action: read_csv | result: fail | error: %v", err)
            continue
        }

		bet := BetFromCSV(record, c.config.ID)
        serializedBet := bet.Serialize()
        betSize := len(serializedBet)

		// Filter bets that exceed the maximum atomic transmission unit
		if betSize > maxBatchBytes {
            log.Errorf("action: filter_bet | result: fail | error: bet exceeds %v bytes", maxBatchBytes)
            continue
        }

		// Check if adding the bet exceeds the byte limit or the count limit
		if currentBatchBytes + betSize > maxBatchBytes || len(batch) >= c.config.MaxAmount {
			log.Infof("action: send_batch | result: in_progress | client_id: %v", c.config.ID)
            if err := c.sendBatchWithRetries(batch); err != nil { return }
            batch = batch[:0]
            currentBatchBytes = 0
        }
		
		batch = append(batch, bet)
        currentBatchBytes += betSize
    }

    // Send remaining records
    if len(batch) > 0 {
        c.sendBatchWithRetries(batch)
    }
	log.Infof("action: loop_finished | result: success | client_id: %v", c.config.ID)
}

// attempts to send a batch, retrying on connection errors
// until success or a termination signal is received.
func (c *Client) sendBatchWithRetries(bets []Bet) error {
    for {
		if c.isStopped() {
			return fmt.Errorf("stop signal received")
		}

        err := c.createClientSocket()
        if err != nil {
            log.Errorf("action: connect | result: fail | error: %v", err)
            time.Sleep(c.config.LoopPeriod)
            continue
        }

        err = c.sendBatchData(bets)
        c.conn.Close()

        if err == nil {
            return nil
        }
        
        log.Errorf("action: send_batch | result: fail | error: %v", err)
        time.Sleep(c.config.LoopPeriod)
    }
}

func (c *Client) sendBatchData(bets []Bet) error {
	var payload []byte
	for _, b := range bets {
		payload = append(payload, b.Serialize()...)
	}

	if err := WriteFrame(c.conn, OpcodeBatch, payload); err != nil {
		return fmt.Errorf("error writing batch: %w", err)
	}

	frame, err := ReadFrame(c.conn)
	if err != nil {
		return fmt.Errorf("error reading server ACK: %w", err)
	}
	
	if frame.Opcode != OpcodeAck {
		return fmt.Errorf("unexpected response opcode: %v", frame.Opcode)
	}

	return nil
}


func (c *Client) openDataset() (*os.File, *csv.Reader, error) {
    filePath := fmt.Sprintf("/data/agency-%s.csv", c.config.ID)
    file, err := os.Open(filePath)
    if err != nil {
        return nil, nil, err
    }
    reader := csv.NewReader(file)
    return file, reader, nil
}

func (c *Client) isStopped() bool {
	select {
	case <-c.stop:
		return true
	default:
		return false
	}
}

func (c *Client) handleShutdown() {
	log.Infof("action: signal_received | result: in_progress | signal: SIGTERM")
	if c.conn != nil {
		c.conn.Close()
	}
	log.Infof("action: client_shutdown | result: success")
}