package main

import (
	"bufio"
	"crypto/sha1"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/artyom/logreader"
	"github.com/artyom/scribe"
	"github.com/artyom/thrift"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

var reconnectForever bool

func main() {
	var port uint
	var stateDir string
	var host, filename string
	var noHostnamePrefix bool

	flag.UintVar(&port, "port", 1463, "scribe port")
	flag.StringVar(&host, "host", "localhost", "scribe host")
	flag.StringVar(&stateDir, "statedir", "", "directory to save state files if none was given in config")
	flag.StringVar(&filename, "conffile", "", "configuration file")
	flag.BoolVar(&noHostnamePrefix, "nohostnameprefix", false, "do not set hostname as a default prefix")
	flag.BoolVar(&reconnectForever, "reconnectforever", false, "try to reconnect forever instead of default 10 retries")

	flag.Parse()

	if filename == "" {
		usage()
		os.Exit(1)
	}

	sections, err := readConfig(filename)
	if err != nil {
		log.Fatal("Failed to read config", err)
	}

	if len(sections) == 0 {
		log.Fatal("Empty configuration, nothing to do")
	}

	l := make(chan *Msg, len(sections))
	go Scriber(host, port, l)

	hostname, err := os.Hostname()
	if err != nil {
		log.Print("Failed to get hostname", err)
		noHostnamePrefix = true
	}

	done := make(chan bool, len(sections))
	for _, cfg := range sections {
		// update empty prefixes if we're ok to use hostname as default one
		if !noHostnamePrefix && cfg.Prefix == "" {
			cfg.Prefix = hostname
		}
		// if no Statefile was given in config and -statedir parameter
		// is present, use auto-generated from pattern statefile names
		if cfg.Statefile == "" && stateDir != "" {
			cfg.Statefile = filepath.Join(stateDir, fmt.Sprintf("logpump-%s.state", signature(cfg.Pattern)))
		}
		go Feeder(cfg, l, done)
	}

	// wait for completion
	for _ = range sections {
		<-done
	}
	log.Print("Bye")
	return
}

func Feeder(cfg Cfg, l chan<- *Msg, done chan<- bool) {
	log.Printf("[%s] Feeder spawned", cfg.Pattern)
	// lines and doInit are channels
	lines, doInit := NewLineEmitter(cfg, done)
	msg := NewMsg("")
	if cfg.Category != "" {
		msg.Category = cfg.Category
	}

FEEDING_LOOP:
	for {
		timeOut := time.NewTimer(2 * time.Minute)
		select {
		case line, ok := <-lines:
			if !ok {
				// something went wrong and we should return
				// (probably logfiles were not found)
				break FEEDING_LOOP
			}
			if cfg.Prefix != "" {
				msg.Text = fmt.Sprintf("%s: %s", cfg.Prefix, line.text)
			} else {
				msg.Text = line.text
			}
			l <- msg
			//log.Print("waiting for status")
			res := <-msg.Status
			//log.Printf("status %+v received", res)
			if res == OK {
				//log.Print("sending true to line.ok")
				line.ok <- true
				//log.Print("true sent to line.ok")
			} else {
				//log.Print("sending false to line.ok")
				line.ok <- false
				//log.Print("false sent to line.ok")
			}
		case <-timeOut.C:
			// re-init if we don't receive any data for a while
			// (trying to detect log rotation)
			doInit <- true
		}
		timeOut.Stop() // stop timer so it can be garbage collected
	}
}

type confirmedLine struct {
	text string
	ok   chan bool
}

func newConfirmedLine() *confirmedLine {
	return &confirmedLine{
		"",
		make(chan bool),
	}
}

// NewLineEmitter abstracts file read operations, position handling, etc.
func NewLineEmitter(cfg Cfg, done chan<- bool) (lines chan *confirmedLine, doInit chan bool) {
	doSave := time.Tick(3 * time.Minute)
	doInit = make(chan bool, 1)
	lines = make(chan *confirmedLine)
	lr, err := NewLogReader(cfg)
	if err != nil {
		// Probably no logfiles were found, no data to feed
		// lines channel with
		log.Print(err)
		close(lines)
		done <- true
		return
	}
	shutdownRequest := make(chan os.Signal, 1)
	signal.Notify(shutdownRequest, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-shutdownRequest
		log.Printf("[%s] Shutdown requested (%s), saving state and finishing", cfg.Pattern, sig)
		lr.SaveState()
		lr.Close()
		done <- true
		return

	}()
	go func() {
		br := bufio.NewReader(lr)
		line := newConfirmedLine()
		firstLine := false
		for {
			select {
			case <-doInit:
				log.Printf("[%s] No new data were found for a while, saving state and re-initing", cfg.Pattern)
				lr.SaveState()
				lr.Close()
				lr.Init()
				br = bufio.NewReader(lr)
			case <-doSave:
				log.Printf("[%s] Periodic state saving", cfg.Pattern)
				lr.SaveState()
			default:
				//log.Print("reading line")
				line.text, err = br.ReadString('\n')
				switch err {
				case nil:
					if firstLine {
						firstLine = false
						lr.state.Signature = signature(line.text)
					}
					for {
						lines <- line
						//log.Print("reading line.ok...")
						res := <-line.ok
						//log.Print("line.ok read")
						if res {
							lr.Confirm(int64(len(line.text)))
							break
						}
						// re-submit failed line after small delay
						time.Sleep(time.Second)
					}
				case io.EOF:
					// next successfully read line would be the first line in file
					if lr.state.Signature == "" {
						firstLine = true
					}
					time.Sleep(time.Millisecond * 300)
				default:
					log.Fatalf("[%s] %s", cfg.Pattern, err)
				}
			}
		}
	}()
	return
}

func Scriber(host string, port uint, l <-chan *Msg) {
	log.Print("Scriber spawned")

	var sentOk, reconnects, retries uint64
	var retryLoopIdx uint8 = 0
	var res scribe.ResultCode
	var sc *client
	var err error
	conns := make(chan *client, 1)

	for {
		sc, err = getScribeClient(host, port)
		if err == nil {
			break
		}
		reconnects += 1
		if !reconnectForever && retryLoopIdx == 10 {
			break
		}
		log.Printf("Trying to reconnect in %d seconds", retryLoopIdx)
		time.Sleep(time.Second * time.Duration(retryLoopIdx))
		if retryLoopIdx < 10 {
			retryLoopIdx += 1
		}
	}
	if err != nil {
		log.Fatalf("Failed to connect to %s:%d (%s)", host, port, err)
	}
	conns <- sc

	ticker := time.Tick(time.Second * 10)
	logentry := scribe.NewLogEntry()
	messages := []*scribe.LogEntry{logentry}
	for {
		select {
		case msg := <-l:
			// push messages to scribe here, handle reconnections, etc.
			// fmt.Print(msg.Text)
			logentry.Message = msg.Text
			logentry.Category = msg.Category
		SENDLOOP:
			for {
				retryLoopIdx = 0
				for {
					select {
					case sc = <-conns:
						// trying to get alive connection from pool
					default:
						reconnects += 1
						sc, err = getScribeClient(host, port)
						if err != nil {
							// skip to reconnect
							// part if we failed to
							// connect
							goto RECONNECT
						}
					}
					res, err = sc.c.Log(messages)
					if err == nil {
						// everything went smoothly,
						// breaking reconnection loop
						break
					} else {
						// something went wrong,
						// closing connection -- it's
						// presumably dead, so we would
						// create new one on next
						// iteration
						sc.t.Close()
					}
				RECONNECT:
					if err != nil {
						log.Print(err)
					}
					// if we don't want to reconnect
					// forever and already have 10
					// unsuccessful retries, break loop and
					// fail
					if !reconnectForever && retryLoopIdx == 10 {
						break
					}
					log.Printf("Trying to reconnect in %d seconds", retryLoopIdx)
					time.Sleep(time.Second * time.Duration(retryLoopIdx))
					// update retryLoopIdx up to 10
					if retryLoopIdx < 10 {
						retryLoopIdx += 1
					}
				}
				if err != nil {
					log.Fatal("Failed to recover connection and send message", err)
				}
				// return connection to pool
				select {
				case conns <- sc:
				default:
				}
				// XXX should be very careful here not to use
				// previous res value
				switch res {
				case scribe.ResultCode_TRY_LATER:
					retries += 1
					time.Sleep(time.Second * 3)
				case scribe.ResultCode_OK:
					msg.Status <- OK
					sentOk += 1
					break SENDLOOP
				}
			}
		case <-ticker:
			log.Printf("STAT: %d messages sent, %d reconnects, %d sent retries", sentOk, reconnects, retries)
		}
	}
}

type Msg struct {
	Text     string
	Category string
	Status   chan status
}

func NewMsg(s string) *Msg {
	return &Msg{s, "default", make(chan status)}
}

type status int

const OK, FAIL status = 0, 1

type Cfg struct {
	Pattern   string
	Statefile string
	Category  string
	Prefix    string
}

type fileMeta struct {
	file      *os.File
	reader    io.Reader
	signature string
}

type State struct {
	Signature string
	Position  int64
}

type LogReader struct {
	cfg         Cfg
	files       []*fileMeta
	currentFile *fileMeta
	state       *State
}

// NewLogReader initializes and returns LogReader object
func NewLogReader(cfg Cfg) (*LogReader, error) {
	lr := new(LogReader)
	lr.cfg = cfg
	lr.state = new(State)
	err := lr.loadState()
	if err != nil {
		log.Printf("[%s] Cannot load state (%s)", cfg.Pattern, cfg.Statefile)
		lr.state.Position = 0
	}
	err = lr.Init()
	if err != nil {
		return nil, err
	}
	// DEBUG
	/*
		for i, x := range lr.files {
			log.Printf("[%d]\t%+v", i, x)
		}
	*/
	return lr, nil
}

// Open & rewind files
func (lr *LogReader) Init() (err error) {
	logfiles, err := filepath.Glob(lr.cfg.Pattern)
	if err != nil {
		return
	}
	if logfiles == nil {
		return fmt.Errorf("Pattern '%s' matched no files", lr.cfg.Pattern)
	}
	sort.Sort(sort.Reverse(logreader.LogNameSlice(logfiles)))

	// re-initialize lr.files - it may not be empty on successive Init() calls
	lr.files = lr.files[:0]

	// try to open all files as fast as we can, skip all heavy operations
	// like getting signature
	for _, f := range logfiles {
		file, err := os.Open(f)
		if err != nil {
			return err
		}
		fm := new(fileMeta)
		fm.file = file
		lr.files = append(lr.files, fm)
	}
	err = lr.getSignatures()
	if err != nil {
		return
	}
	if len(lr.files) == 0 {
		log.Printf("[%s] Matched no non-empty files, would re-try in a while", lr.cfg.Pattern)
		return nil
	}

	// close and forget already read files (to the left of the file with
	// matching signature)
	oldFileFound := false
	idx := 0
	for i, fm := range lr.files {
		if lr.state.Signature != "" && lr.state.Signature == fm.signature {
			for _, fm2 := range lr.files[:i] {
				fm2.file.Close()
			}
			idx = i
			oldFileFound = true
			break
		}
	}
	if idx > 0 {
		lr.files = lr.files[idx:]
	}

	// rewind current file & create reading wrapper
	if oldFileFound && lr.state.Signature != "" && lr.state.Position > 0 {
		if !strings.HasSuffix(lr.files[0].file.Name(), ".gz") && !strings.HasSuffix(lr.files[0].file.Name(), ".bz2") {
			lr.files[0].file.Seek(lr.state.Position, os.SEEK_SET)
			lr.files[0].reader = io.Reader(lr.files[0].file)
		} else {
			r, err := logreader.NewPlainTextReader(lr.files[0].file)
			if err != nil {
				log.Print("Failed to unpack", lr.files[0].file.Name(), err)
				return err
			}
			_, err = io.CopyN(ioutil.Discard, r, lr.state.Position)
			// TODO check whether written to Discard == Position
			if err != nil {
				log.Print("Failed to rewind", lr.files[0].file.Name(), err)
				return err
			}
			lr.files[0].reader = r
		}
	} else {
		r, err := logreader.NewPlainTextReader(lr.files[0].file)
		if err != nil {
			log.Print("Failed to unpack", lr.files[0].file.Name(), err)
			return err
		}
		lr.files[0].reader = r
	}
	// create reading wrappers for the rest of the files
	for _, f := range lr.files[1:] {
		r, err := logreader.NewPlainTextReader(f.file)
		if err != nil {
			log.Print("Failed to unpack", f.file.Name(), err)
			return err
		}
		f.reader = r
	}
	// reset previously loaded state if that file cannot be found
	if !oldFileFound && len(lr.files) > 0 {
		lr.state.Signature = lr.files[0].signature
		lr.state.Position = 0
	}

	// all ok, ready to read data
	return nil
}

// get signature from open files
func (lr *LogReader) getSignatures() (err error) {

	var nonEmptyFiles []*fileMeta

	for _, fm := range lr.files {
		sig, err := getSignature(fm.file)
		if err == io.EOF {
			log.Printf("Skipping empty file %s", fm.file.Name())
			fm.file.Close()
			continue
		}
		if err != nil {
			log.Print("Error reading signature from file", fm.file.Name())
			return err
		}
		fm.signature = sig
		nonEmptyFiles = append(nonEmptyFiles, fm)
	}
	lr.files = nonEmptyFiles
	return
}

// Load state from file
func (lr *LogReader) loadState() (err error) {
	if lr.cfg.Statefile == "" {
		return fmt.Errorf("Neither Statefile nor -statedir option was given")
	}
	dat, err := ioutil.ReadFile(lr.cfg.Statefile)
	if err != nil {
		return
	}
	st := new(State)
	err = json.Unmarshal(dat, st)
	if err != nil {
		log.Print("Cannot unmarshal state from statefile", lr.cfg.Statefile, err)
	}
	lr.state = st
	return
}

// Dump state to file
func (lr *LogReader) SaveState() (err error) {
	if lr.cfg.Statefile == "" {
		return fmt.Errorf("No state file specified, nowhere to save")
	}
	dat, err := json.Marshal(lr.state)
	if err != nil {
		return
	}
	err = ioutil.WriteFile(lr.cfg.Statefile, dat, 0644)
	if err != nil {
		log.Print("Cannot save state to", lr.cfg.Statefile, err)
	}
	return
}

// Increase position
func (lr *LogReader) Confirm(n int64) int64 {
	return atomic.AddInt64(&lr.state.Position, n)
}

// Implementing io.Reader interface
// XXX this won't work as we have both gzipped and plain text files
// we should operate on wrapping readers instead
func (lr *LogReader) Read(b []byte) (n int, err error) {
	// we don't have any files to read yet (only empty files matched
	// pattern, should wait for next re-init call)
	if len(lr.files) == 0 {
		return 0, io.EOF
	}
	for len(lr.files) > 1 {
		// DEBUG
		//log.Printf("%T, %+v", lr.files[0], lr.files[0])
		n, err = lr.files[0].reader.Read(b)
		if n > 0 || err != io.EOF {
			if err == io.EOF {
				err = nil
			}
			return
		}
		lr.files[0].file.Close()
		lr.files = lr.files[1:]
		// TODO there's a tiny chance of race condition between
		// changing this two lr.state fields and reading them in
		// SaveState() method. It's probably better to protect these
		// with lock
		lr.state.Signature = lr.files[0].signature
		atomic.StoreInt64(&lr.state.Position, 0)
	}
	// XXX check if this works correctly
	n, err = lr.files[0].reader.Read(b)
	return
}

// Close any files associated with LogReader
func (lr *LogReader) Close() error {
	for _, f := range lr.files {
		f.file.Close()
	}
	return nil
}

func getSignature(f *os.File) (string, error) {
	r, err := logreader.NewPlainTextReader(f)
	if err != nil {
		return "", err
	}
	s, err := bufio.NewReader(r).ReadString('\n')
	if err != nil {
		return "", err
	}
	f.Seek(0, os.SEEK_SET)
	return signature(s), nil
}
func signature(s string) string {
	h := sha1.New()
	io.WriteString(h, s)
	return fmt.Sprintf("%x", h.Sum(nil))
}

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s -host <host> -port <port> -conffile <config.json>\n", path.Base(os.Args[0]))
	flag.PrintDefaults()
}

func readConfig(filename string) ([]Cfg, error) {
	dat, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	conf := []Cfg{}
	err = json.Unmarshal(dat, &conf)
	if err != nil {
		return nil, err
	}
	conf2 := []Cfg{}
	for _, v := range conf {
		if v.Pattern != "" {
			conf2 = append(conf2, v)
		}
	}
	return conf2, nil
}

type client struct {
	c *scribe.ScribeClient
	t thrift.TTransport
}

func getScribeClient(host string, port uint) (c *client, err error) {

	var tf thrift.TTransportFactory
	var transport thrift.TTransport

	hostPort := fmt.Sprintf("%s:%d", host, port)

	transport, err = thrift.NewTSocketTimeout(hostPort, time.Duration(5)*time.Second)
	if err != nil {
		return
	}

	tf = thrift.NewTTransportFactory()
	tf = thrift.NewTFramedTransportFactory(tf)
	transport = tf.GetTransport(transport)
	//defer transport.Close()
	err = transport.Open()
	if err != nil {
		return
	}

	protofac := thrift.NewTBinaryProtocolFactory(false, false)
	scribeClient := scribe.NewScribeClientFactory(transport, protofac)

	c = &client{scribeClient, transport}
	return
}
