package storage

import (
	"io"

	"github.com/libp2p/go-libp2p/core/network"
	"go.uber.org/zap"
)

// HandleStorageStream handles incoming storage protocol streams
func (s *Service) HandleStorageStream(stream network.Stream) {
	defer stream.Close()

	// Read request
	data, err := io.ReadAll(stream)
	if err != nil {
		s.logger.Error("Failed to read storage request", zap.Error(err))
		return
	}

	var request StorageRequest
	if err := request.Unmarshal(data); err != nil {
		s.logger.Error("Failed to unmarshal storage request", zap.Error(err))
		return
	}

	// Process request
	response := s.processRequest(&request)

	// Send response
	responseData, err := response.Marshal()
	if err != nil {
		s.logger.Error("Failed to marshal storage response", zap.Error(err))
		return
	}

	if _, err := stream.Write(responseData); err != nil {
		s.logger.Error("Failed to write storage response", zap.Error(err))
		return
	}

	s.logger.Debug("Handled storage request",
		zap.String("type", string(request.Type)),
		zap.String("key", request.Key),
		zap.String("namespace", request.Namespace),
		zap.Bool("success", response.Success),
	)
}
