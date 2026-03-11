package daemon

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"syscall"

	"ctm/internal/protocol"
)

type Server struct {
	sockPath string
	lockPath string
	hub      *Hub
	listener net.Listener
	lockFile *os.File
}

func NewServer(sockPath, lockPath, sessionsDir, collectionsDir, bookmarksDir, overlaysDir, workspacesDir, savedSearchesDir, syncCloudDir, searchIndexPath string) *Server {
	return &Server{
		sockPath: sockPath,
		lockPath: lockPath,
		hub:      NewHub(sessionsDir, collectionsDir, bookmarksDir, overlaysDir, workspacesDir, savedSearchesDir, syncCloudDir, searchIndexPath),
	}
}

func (s *Server) Start(ctx context.Context) error {
	// Acquire flock for singleton enforcement
	lockFile, err := acquireLock(s.lockPath)
	if err != nil {
		return err
	}
	s.lockFile = lockFile

	// Remove stale socket if it exists
	os.Remove(s.sockPath)

	listener, err := net.Listen("unix", s.sockPath)
	if err != nil {
		s.lockFile.Close()
		return fmt.Errorf("listen %s: %w", s.sockPath, err)
	}
	s.listener = listener

	// Verify socket is not a symlink (defense against symlink attack)
	if fi, err := os.Lstat(s.sockPath); err == nil {
		if fi.Mode()&os.ModeSymlink != 0 {
			listener.Close()
			s.lockFile.Close()
			return fmt.Errorf("socket path is a symlink, refusing to start")
		}
	}

	// Set socket permissions
	if err := os.Chmod(s.sockPath, 0600); err != nil {
		log.Printf("[daemon %s] warning: chmod socket: %v", timeStr(), err)
	}

	log.Printf("[daemon %s] listening on %s", timeStr(), s.sockPath)

	// Start Hub in background
	go s.hub.Run(ctx)

	// Accept loop
	go func() {
		for {
			conn, err := s.listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					return
				default:
					log.Printf("[daemon %s] accept error: %v", timeStr(), err)
					continue
				}
			}
			go s.handleConnection(ctx, conn)
		}
	}()

	// Wait for shutdown
	select {
	case <-ctx.Done():
	case <-s.hub.stopCh:
	}

	return s.shutdown()
}

func (s *Server) shutdown() error {
	log.Printf("[daemon %s] shutting down", timeStr())

	if s.listener != nil {
		s.listener.Close()
	}
	os.Remove(s.sockPath)

	if s.lockFile != nil {
		s.lockFile.Close()
		os.Remove(s.lockPath)
	}

	return nil
}

func (s *Server) handleConnection(_ context.Context, conn net.Conn) {
	reader := protocol.NewReader(conn)
	writer := protocol.NewWriter(conn)

	// Register with Hub
	s.hub.registerCh <- &connRegistration{conn: conn, writer: writer}

	// Read loop
	for {
		msg, err := reader.Read()
		if err != nil {
			s.hub.unregisterCh <- conn
			return
		}

		s.hub.messageCh <- &incomingMessage{
			conn:   conn,
			writer: writer,
			msg:    msg,
		}
	}
}

func acquireLock(path string) (*os.File, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open lock file: %w", err)
	}

	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("daemon already running (lock: %s)", path)
	}

	return f, nil
}
