package tunnel


import (
	"log"
	"os"
	"runtime"
	"sync"
	"time"
	"github.com/pkg/errors"
)

var (
	logger = log.New(os.Stdout, "tunnel: ", log.LstdFlags | log.Lshortfile)
)

// message handler
type MessageHandler func(t *Tunnel, msg Message) (bool, error) 

// port event
type PortEvent struct {
	// action
	Action int
	// port
	Port *Port
}

// tunnel
type Tunnel struct {
	// tunnel lock
	lock sync.Mutex
	// input mesages
	input chan Message
	// ports
	ports map[string]*Port
	// fallback port
	fallback *Port
	// quit
	q chan bool
	// error
	e chan error
	// handlers
	handlers []MessageHandler
}


func NewTunnel() (*Tunnel) {
	return &Tunnel{
		input: make (chan Message),
		ports: make (map[string]*Port),
		q: make (chan bool),
		e: make (chan error),
	}
}

func NewTunSwitch() (*Tunnel) {
	t := NewTunnel()
	// add tun logic
	t.AddMessageHandler(Tun)
	// add tun fallback port
	// t.AddPort("0.0.0.0", true)
	return t
}


func (t *Tunnel) AddMessageHandler(h MessageHandler) {
	t.handlers = append(t.handlers, h)
}

// add port with address
func (t *Tunnel) AddPort(addr string, fallback bool) (*Port, error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	p, _ := t.ports[addr]
	if p != nil {
		logger.Printf("port(%s) exists, remove the old one", addr)
		p.Close()
	}
	p = &Port{
		Addr: addr,
		IsFallback: fallback,
		input: t.input,
		output: make(chan Message),
		t: t,
	}
	t.ports[addr] = p
	// register fallbck port
	if fallback {
		t.fallback = p
	}
	return p, nil
}

// remove port
func (t *Tunnel) RemovePort(addr string) (error) {
	t.lock.Lock()
	defer t.lock.Unlock()
	delete(t.ports, addr)	
	return nil
}

// get port by addr
func (t *Tunnel) GetPort(addr string) *Port {
	t.lock.Lock()
	defer t.lock.Unlock()
	p, _ := t.ports[addr]
	return p
}

// get the fallback port
func (t *Tunnel) GetFallbackPort() *Port {
	return t.fallback
}

// close tunnel
func (t *Tunnel) Close() (error) {
	t.q <- true
	return nil
}

// start tunnel logic
func (t *Tunnel) Start() (chan error) {
	go func() {
		t.e <- t.run()
	} ()
	return t.e
}

// tunnel main loop
func (t *Tunnel) run() (error) {

	for {
		select {
			case msg := <- t.input:
				// handle inbound messages
				for i := len(t.handlers)-1; i >= 0; i--  {
					h := t.handlers[i]
					done, err := h(t, msg)
					if err != nil {
						return errors.Wrap(err, "")
					}
					if done {
						break
					}
				}
			case <- t.q:
				// try close all the ports
				for addr, port := range t.ports {
					logger.Printf("closing port %s", addr)
					port.Close()
				}
				// quit
				logger.Printf("tunnel closed")
				return errors.New("quit")
			case <- time.Tick(time.Second * 30):
				// nothing
				logger.Println("goroutines:", runtime.NumGoroutine())
		}
	}
	return nil
}
