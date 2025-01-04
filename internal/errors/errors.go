package errors

import (
	"fmt"
	"os"
	"time"
)

type ErrorType int

var Errorchan = make(chan *AppError, 100)

const (
	ErrConsent ErrorType = iota
	ErrFileTransfer
	ErrConnection
	ErrPeer
)

type ErrorLevel int

const (
	INFO ErrorLevel = iota
	WARNING
	ERROR
	FATAL
)

type AppError struct {
	Type    ErrorType
	Level   ErrorLevel
	Message string
	Time    time.Time
	Source  string
	Err     error
}

func (c *AppError) Error() string {
	return c.Message
}

func NewError(errtype ErrorType, level ErrorLevel, source string, msg string, uerror error) *AppError {
	err := &AppError{
		Type:    errtype,
		Level:   level,
		Message: msg,
		Time:    time.Now(),
		Source:  source,
		Err:     uerror,
	}

	//we are sending the custom error to the error channel with a buffer of 100
	// Errorchan <- err
	fmt.Print("\r\033[K")
	fmt.Fprint(os.Stdout, msg)
	return err
}
