package mikrotik

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/go-routeros/routeros/v3"
)

const defaultMikrotikAPIPort = "8728"
const DefaultTimeout = 10 * time.Second

// Client talks to a RouterOS device over the API.
type Client struct {
	Address  string
	Username string
	Password string
	Timeout  time.Duration

	mu      sync.Mutex
	client  *routeros.Client
	ctx     context.Context
	semHeld bool
	closed  bool
}

func NewClient(address, username, password string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Client{
		Address:  address,
		Username: username,
		Password: password,
		Timeout:  timeout,
		ctx:      context.Background(),
	}
}

// BeginScrape installs a per-scrape context with the configured timeout.
// Call Close() when the scrape finishes to release the connection slot.
func (c *Client) BeginScrape(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithTimeout(parent, c.Timeout)
	c.mu.Lock()
	c.ctx = ctx
	c.closed = false
	c.mu.Unlock()
	return ctx, cancel
}

func (c *Client) scrapeContext() context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

func (c *Client) Connect() error {
	return c.ConnectContext(c.scrapeContext())
}

func (c *Client) ConnectContext(ctx context.Context) error {
	c.mu.Lock()
	if c.client != nil && !c.closed {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	if err := acquireConn(ctx); err != nil {
		return fmt.Errorf("connection limit: %w", err)
	}

	addr := c.Address
	if _, _, err := net.SplitHostPort(addr); err != nil {
		addr = net.JoinHostPort(addr, defaultMikrotikAPIPort)
	}

	log.Printf("Connecting to MikroTik router at %s...", addr)
	cli, err := routeros.DialContext(ctx, addr, c.Username, c.Password)
	if err != nil {
		releaseConn()
		log.Printf("Error dialing MikroTik router %s: %v", addr, err)
		return err
	}

	c.mu.Lock()
	c.client = cli
	c.semHeld = true
	c.closed = false
	c.ctx = ctx
	c.mu.Unlock()
	return nil
}

func (c *Client) Close() {
	c.mu.Lock()
	cli := c.client
	held := c.semHeld
	c.client = nil
	c.semHeld = false
	c.closed = true
	c.mu.Unlock()

	if cli != nil {
		log.Printf("Closing connection to MikroTik router %s", c.Address)
		cli.Close()
	}
	if held {
		releaseConn()
	}
}

func (c *Client) api() (*routeros.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.client == nil {
		return nil, fmt.Errorf("client not connected")
	}
	return c.client, nil
}

func (c *Client) Run(cmd ...string) (*routeros.Reply, error) {
	return c.RunContext(c.scrapeContext(), cmd...)
}

func (c *Client) RunArgs(args []string) (*routeros.Reply, error) {
	return c.RunArgsContext(c.scrapeContext(), args)
}

func (c *Client) RunContext(ctx context.Context, cmd ...string) (*routeros.Reply, error) {
	return c.run(ctx, "command", func(cli *routeros.Client) (*routeros.Reply, error) {
		return cli.RunContext(ctx, cmd...)
	})
}

func (c *Client) RunArgsContext(ctx context.Context, args []string) (*routeros.Reply, error) {
	return c.run(ctx, "command with args", func(cli *routeros.Client) (*routeros.Reply, error) {
		return cli.RunArgsContext(ctx, args)
	})
}

// run connects if needed, executes fn, and closes the client when the context is cancelled.
func (c *Client) run(ctx context.Context, kind string, fn func(*routeros.Client) (*routeros.Reply, error)) (*routeros.Reply, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	c.mu.Lock()
	needConnect := c.client == nil || c.closed
	c.mu.Unlock()
	if needConnect {
		if err := c.ConnectContext(ctx); err != nil {
			return nil, err
		}
	}

	cli, err := c.api()
	if err != nil {
		return nil, err
	}

	reply, err := fn(cli)
	if err != nil {
		log.Printf("Error running %s on %s: %v", kind, c.Address, err)
		if ctx.Err() != nil {
			c.Close()
		}
		return nil, err
	}
	return reply, nil
}
