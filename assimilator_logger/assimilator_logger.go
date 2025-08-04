package assimilator_logger

import (
	"fmt"
	"log"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ANSI escape codes for text formatting

const (
	Overwrite = "\r"       // Carriage return to overwrite the current line
	Lgreen    = "\033[92m" // Light Green
	Lblack    = "\033[90m" // Light Black (or Dark Gray)
	Lred      = "\033[91m" // Light Red
	Lblue     = "\033[94m" // Light Blue
	White     = "\033[97m"
	Reset     = "\033[0m" // Reset to default color
)

type LogLevel int

var ProgramIsClosing bool = false

const (
	LevelSilent    LogLevel = 0 // Nothing logged, only for setting minimum verbosity
	LevelFatal     LogLevel = 1 // Fatal always shows (or causes exit)
	LevelUnhandled LogLevel = 1 // Errors I've not yet experienced.
	LevelError     LogLevel = 2 // Errors are critical, show at base level
	LevelWarning   LogLevel = 3 // Warnings show at level 3 and above
	LevelInfo      LogLevel = 4 // Default level for general messages
	LevelSuccess   LogLevel = 4 // Success messages typically show at base level
	LevelDebug     LogLevel = 5 // Debug messages show at level 5 and above
	LevelTrace     LogLevel = 6 // Very verbose messages show at level 6 and above
)

var LogType = map[string]func(LogEntry){
	"console":   consoleLog,
	"file":      fileLog,
	"maas":      maasLog,
	"logserver": logServer,
	"slack":     slackLog,
}

var logFile *os.File

type LogEntry struct {
	level       LogLevel
	message     string
	color       string
	flatPrefix  string
	emojiPrefix string
	IsFatal     bool
	ExitCode    int
}

// Global logger instance (initialized at package level)
var globalLogger *AssLogger

func StartLogger() {
	globalLogger = NewAssLogger()
	globalLogger.startWorker()
}

// SetVerbosity sends a new verbosity level to the running logger.
// This is the function you'll call after parsing flags.
func SetVerbosity(level int) {
	if globalLogger != nil {
		globalLogger.verbosityUpdates <- level
	}
}

func SetLogTypes(logTypes map[string]bool) {
	if globalLogger != nil && logTypes != nil {
		globalLogger.logTypesUpdates <- logTypes
	}
}

type LogWriter struct {
}

// Implement the io.Writer interface for LogWriter ---
// This is the "Write button" that go-git will press.
func (l *LogWriter) Write(p []byte) (n int, err error) {
	log := string(p[:])
	Debug(log)
	return len(p), nil
}

// Exported function to get a LogWriter instance (optional, but good practice) ---
func NewLogWriter() *LogWriter {
	return &LogWriter{}
}

type AssLogger struct {
	msgBuffer        chan LogEntry
	verbosityUpdates chan int // Channel to receive verbosity level updates
	logTypesUpdates  chan map[string]bool
	wg               sync.WaitGroup
	closeSync        sync.Once
	verbosityLevel   LogLevel // Local verbosity level for the logger instance
	logTypes         map[string]bool
}

func NewAssLogger() *AssLogger {
	return &AssLogger{
		msgBuffer:        make(chan LogEntry, 1000),
		verbosityUpdates: make(chan int, 1),
		verbosityLevel:   LevelTrace,
		logTypesUpdates:  make(chan map[string]bool, 1),
		logTypes:         map[string]bool{"console": true},
	}
}

func (l *AssLogger) startWorker() {
	l.wg.Add(1)

	go func() {
		defer l.wg.Done()

		for {
			select {
			// Case 1: A new verbosity level is received.
			case newLevel := <-l.verbosityUpdates:
				// Clamp the value to the valid range of LogLevel
				newLevel = max(newLevel, int(LevelSilent))
				newLevel = min(newLevel, int(LevelTrace))
				l.verbosityLevel = LogLevel(newLevel)

			case newLogTypes := <-l.logTypesUpdates:
				l.logTypes = newLogTypes
			// Case 3: A new log entry is received.
			case entry, ok := <-l.msgBuffer:
				if !ok {
					// The channel was closed. Exit the goroutine.
					return
				}
				// Check if the entry's level is within the current verbosity threshold.
				if entry.level <= l.verbosityLevel {
					l.outputLog(entry)
				}
				if entry.IsFatal {
					time.Sleep(1000 * time.Millisecond)
					os.Exit(entry.ExitCode)
				}
			}
		}
	}()
}

func Close(ExitCode ...int) {
	globalLogger.Close()
	if len(ExitCode) > 0 {
		os.Exit(ExitCode[0])
	}
	os.Exit(0)
}

func getCallerInfo(skip int) string {
	pc, file, line, ok := runtime.Caller(skip)
	if !ok {
		return "???:0"
	}

	fn := runtime.FuncForPC(pc)
	if fn == nil {
		return fmt.Sprintf("%s:%d", path.Base(file), line)
	}

	// This gets the full name, e.g., "github.com/geogian28/Assimilator/main.isRoot"
	fullFuncName := fn.Name()

	// We just want the "main.isRoot" part
	baseFuncName := path.Base(fullFuncName)

	// Now, we want to strip the package name "main." to get just "isRoot"
	lastDotIndex := strings.LastIndex(baseFuncName, ".")
	if lastDotIndex == -1 {
		// This case handles functions without a package qualifier (unlikely in most Go code)
		return fmt.Sprintf("[%s: %d] ", baseFuncName, line)
	}
	shortFuncName := baseFuncName[lastDotIndex+1:]

	return fmt.Sprintf("[%s: %d] ", shortFuncName, line)
}

func (l *AssLogger) Close() {
	Info("Closing down now!")
	ProgramIsClosing = true

	l.closeSync.Do(func() {
		close(l.msgBuffer)
	})
	l.wg.Wait()
}

// Chooses outputs to multiple locations based on logTypes
func (l *AssLogger) outputLog(entry LogEntry) {
	for logType := range l.logTypes {
		LogType[logType](entry)
	}
}

// Private helper to print formatted messages
func consoleLog(entry LogEntry) {
	// Add timestamp here if you want it to be common
	// fmt.Printf("%s %s %s%s\n", time.Now().Format("15:04:05"), prefix, color, message)
	//fmt.Printf("%s%s %s%s\n", Overwrite, prefix, color, message) // Using your Overwrite
	log.Printf("%s%s %s%s%s\n", Overwrite, entry.color, entry.emojiPrefix, entry.message, Reset) // Using your Overwrite
}

func fileLog(entry LogEntry) {
	if logFile == nil {
		var err error
		logFile, err = os.OpenFile("/var/log/assimilator.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Println("Failed to open log file:", err)
		}
	} else {
		_, err := fmt.Fprintf(logFile, "%s %s\n", entry.flatPrefix, entry.message)

		if err != nil {
			fmt.Println("Failed to write log file:", err)
		}
	}
}

func maasLog(entry LogEntry) {
	fmt.Println("maasLog")
}

func logServer(entry LogEntry) {
	fmt.Println("logServer")
}

func slackLog(entry LogEntry) {
	fmt.Println("slackLog")
}

func (l *AssLogger) Debug(message string) {
	if ProgramIsClosing {
		return
	}
	globalLogger.msgBuffer <- LogEntry{
		level:       LevelInfo,
		message:     message,
		color:       Lblue,
		flatPrefix:  "[Debug  ] ",
		emojiPrefix: "[🪲 Debug ] ",
	}
}

func (l *AssLogger) Trace(message string) {
	if ProgramIsClosing {
		return
	}
	globalLogger.msgBuffer <- LogEntry{
		level:       LevelInfo,
		message:     message,
		color:       Lblack,
		flatPrefix:  "[Trace  ] ",
		emojiPrefix: "[🫆 Trace  ] ",
	}
}
func (l *AssLogger) Info(message string) {
	if ProgramIsClosing {
		return
	}
	globalLogger.msgBuffer <- LogEntry{
		level:       LevelInfo,
		message:     message,
		color:       White,
		flatPrefix:  "[Info   ] ",
		emojiPrefix: "[ℹ️ Info   ] ",
	}
}

func (l *AssLogger) Success(message string) {
	if ProgramIsClosing {
		return
	}
	globalLogger.msgBuffer <- LogEntry{
		level:       LevelSuccess,
		message:     message,
		color:       Lgreen,
		flatPrefix:  "[Success] ",
		emojiPrefix: "[✅ Success] ",
	}
}

func (l *AssLogger) Warning(message string) {
	if ProgramIsClosing {
		return
	}
	globalLogger.msgBuffer <- LogEntry{
		level:       LevelWarning,
		message:     message,
		color:       Lred,
		flatPrefix:  "[Warn   ] ",
		emojiPrefix: "[❗ Warn   ] ",
	}
}

func (l *AssLogger) Error(message string) {
	if ProgramIsClosing {
		return
	}
	globalLogger.msgBuffer <- LogEntry{
		level:       LevelError,
		message:     message,
		color:       Lred,
		flatPrefix:  "[ Error  ] ",
		emojiPrefix: "[❌ Error ] ",
	}
}

func (l *AssLogger) Fatal(exitCode int, message string) {
	if ProgramIsClosing {
		return
	}
	globalLogger.msgBuffer <- LogEntry{
		level:       LevelFatal,
		message:     message,
		color:       Lred,
		flatPrefix:  "[Fatal  ] ",
		emojiPrefix: "[💀 Fatal  ] ",
		IsFatal:     true,
		ExitCode:    exitCode,
	}
	l.wg.Wait()
}

func (l *AssLogger) Unhandled(message string) {
	if ProgramIsClosing {
		return
	}
	globalLogger.msgBuffer <- LogEntry{
		level:       LevelUnhandled,
		message:     message,
		color:       Lred,
		flatPrefix:  "[unhandled error] ",
		emojiPrefix: "[💥 unhandled error] ",
		IsFatal:     true,
		ExitCode:    28,
	}
	l.wg.Wait()
}

// Convenience functions for direct use (e.g., utils.Info("msg"))
// These essentially wrap the methods of the global logger instance.
func Debug(messages ...any) {
	message := fmt.Sprint(messages...)
	message = getCallerInfo(2) + message
	globalLogger.Debug(message)
}

func Trace(messages ...any) {
	message := fmt.Sprint(messages...)
	message = getCallerInfo(2) + message
	globalLogger.Trace(message)
}

func Info(messages ...any) {
	message := fmt.Sprint(messages...)
	message = getCallerInfo(2) + message
	globalLogger.Info(message)
}

func Success(messages ...any) {
	message := fmt.Sprint(messages...)
	message = getCallerInfo(2) + message
	globalLogger.Success(message)
}

func Warning(messages ...any) {
	message := fmt.Sprint(messages...)
	message = getCallerInfo(2) + message
	globalLogger.Warning(message)
}

func Error(messages ...any) {
	message := fmt.Sprint(messages...)
	message = getCallerInfo(2) + message
	globalLogger.Error(message)
}

func Fatal(exitcode int, messages ...any) {
	message := fmt.Sprint(messages...)
	message = getCallerInfo(2) + message
	globalLogger.Fatal(exitcode, message)
	time.Sleep(200 * time.Millisecond)
}

func Unhandled(messages ...any) {
	message := fmt.Sprint(messages...)
	message = getCallerInfo(2) + message
	globalLogger.Unhandled(message)
	time.Sleep(200 * time.Millisecond)
}
