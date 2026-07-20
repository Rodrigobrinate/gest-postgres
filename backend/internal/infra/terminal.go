package infra

import (
	"context"

	"github.com/docker/docker/api/types"
)

// OpenTerminal abre um exec interativo (Tty) dentro do container — thin
// wrapper sobre docker.Client, exposto pro handler WebSocket (ver
// internal/api/terminal.go) montar o pipe stdin/stdout.
func (s *Service) OpenTerminal(ctx context.Context, containerID string) (execID string, hijacked types.HijackedResponse, err error) {
	execID, err = s.docker.ExecCreate(ctx, containerID, nil)
	if err != nil {
		return "", types.HijackedResponse{}, err
	}
	hijacked, err = s.docker.ExecAttach(ctx, execID)
	if err != nil {
		return "", types.HijackedResponse{}, err
	}
	return execID, hijacked, nil
}

func (s *Service) ResizeTerminal(ctx context.Context, execID string, rows, cols uint) error {
	return s.docker.ExecResize(ctx, execID, rows, cols)
}
