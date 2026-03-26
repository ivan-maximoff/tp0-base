package common

import (
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"strings"
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

		batch, currentBatchBytes = c.processRecordAndBatch(record, batch, currentBatchBytes, maxBatchBytes)
    }

    // Send remaining records
    if len(batch) > 0 {
        c.sendBatchWithRetries(batch)
    }
	log.Infof("action: loop_finished | result: success | client_id: %v", c.config.ID)

	// Send end of data notification
	if err := c.sendNotification(OpcodeEndData); err != nil {
        log.Errorf("action: end_data | result: fail | error: %v", err)
        return
    }

	// Wait for the lottery to finish
    c.queryWinners()
}

func (c *Client) processRecordAndBatch(record []string, batch []Bet, currentBatchBytes int, maxBatchBytes int) ([]Bet, int) {
	bet := BetFromCSV(record, c.config.ID)
	serializedBet := bet.Serialize()
	betSize := len(serializedBet)

	// Filter bets that exceed the maximum atomic transmission unit
	if betSize > maxBatchBytes {
		log.Errorf("action: filter_bet | result: fail | error: bet exceeds %v bytes", maxBatchBytes)
		return batch, currentBatchBytes
	}

	// Check if adding the bet exceeds the byte limit or the count limit
	if currentBatchBytes + betSize > maxBatchBytes || len(batch) >= c.config.MaxAmount {
		log.Infof("action: send_batch | result: in_progress | client_id: %v", c.config.ID)
		if err := c.sendBatchWithRetries(batch); err != nil { 
			// Stop accumulating if retry failed (e.g. stopped)
			return batch[:0], 0
		}
		batch = batch[:0]
		currentBatchBytes = 0
	}
	
	batch = append(batch, bet)
	currentBatchBytes += betSize
	return batch, currentBatchBytes
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

func (c *Client) sendNotification(opcode byte) error {
    if err := c.createClientSocket(); err != nil { return err }
    defer c.conn.Close()

    if err := WriteFrame(c.conn, opcode, []byte(c.config.ID)); err != nil { return err }
    _, err := ReadFrame(c.conn)
    return err
}

// Performs a polling mechanism to fetch winners once the lottery is done.
func (c *Client) queryWinners() {
    for {
        if c.isStopped() { return }

        if err := c.createClientSocket(); err != nil {
            time.Sleep(c.config.LoopPeriod)
            continue
        }

        err := WriteFrame(c.conn, OpcodeGetWinners, []byte(c.config.ID))
        if err == nil {
            frame, errRead := ReadFrame(c.conn)
            if errRead == nil && frame.Opcode == OpcodeAck {
				c.handleWinnersResponse(frame.Body)
				c.conn.Close()
				return
			}
        }
        
        c.conn.Close()
        select {
        case <-time.After(c.config.LoopPeriod): 
            continue
        case <-c.stop:
            return
        }
    }
}

// Parses the winning DNIs and logs the final result.
func (c *Client) handleWinnersResponse(data []byte) {
	winners := []string{}
	if len(data) > 0 {
		winners = strings.Split(string(data), ",")
	}
	log.Infof("action: consulta_ganadores | result: success | cant_ganadores: %d", len(winners))
}