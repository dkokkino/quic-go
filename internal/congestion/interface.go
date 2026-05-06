package congestion

import (
	"time"

	"github.com/quic-go/quic-go/internal/protocol"
	"github.com/quic-go/quic-go/internal/utils"
)

// A SendAlgorithm performs congestion control.
// All timestamps use stdlib time.Time so implementations can live outside this module.
type SendAlgorithm interface {
	TimeUntilSend(bytesInFlight protocol.ByteCount) time.Time
	HasPacingBudget(now time.Time) bool
	OnPacketSent(sentTime time.Time, bytesInFlight protocol.ByteCount, packetNumber protocol.PacketNumber, bytes protocol.ByteCount, isRetransmittable bool)
	CanSend(bytesInFlight protocol.ByteCount) bool
	MaybeExitSlowStart()
	OnPacketAcked(number protocol.PacketNumber, ackedBytes protocol.ByteCount, priorInFlight protocol.ByteCount, eventTime time.Time)
	OnCongestionEvent(number protocol.PacketNumber, lostBytes protocol.ByteCount, priorInFlight protocol.ByteCount)
	SetMaxDatagramSize(protocol.ByteCount)
}

// A SendAlgorithmWithRTTStats is a SendAlgorithm that supports late RTT stats binding.
type SendAlgorithmWithRTTStats interface {
	SendAlgorithm
	SetRTTStats(*utils.RTTStats)
}

// A SendAlgorithmWithDebugInfos is a SendAlgorithm that exposes some debug infos
type SendAlgorithmWithDebugInfos interface {
	SendAlgorithm
	InSlowStart() bool
	InRecovery() bool
	GetCongestionWindow() protocol.ByteCount
}

// AckEventHandler is implemented by congestion controllers that need ACK-event
// boundaries instead of only per-packet callbacks.
// Uses stdlib time.Time so implementations can live outside the quic-go module.
type AckEventHandler interface {
	OnAckEventStart(eventTime time.Time, bytesInFlight protocol.ByteCount)
	OnAckEventEnd(eventTime time.Time)
}

// LossDetectionHandler is implemented by congestion controllers that need a
// callback before each loss-detection pass.
type LossDetectionHandler interface {
	OnLossDetectionStart()
}

// ECNFeedbackHandler is implemented by congestion controllers that consume
// QUIC ACK-frame ECN counters directly.
// Uses stdlib time.Time so implementations can live outside the quic-go module.
type ECNFeedbackHandler interface {
	OnECNFeedback(
		ackedBytes protocol.ByteCount,
		ect0Total, ect1Total, ceTotal int64,
		priorInFlight protocol.ByteCount,
		eventTime time.Time,
	)
}

// AppLimitedHandler is implemented by congestion controllers that track
// app-limited bubbles explicitly.
type AppLimitedHandler interface {
	MarkAppLimited(bytesInFlight protocol.ByteCount)
}

// SpuriousLossHandler is implemented by congestion controllers that can react
// to packets that were spuriously declared lost.
type SpuriousLossHandler interface {
	OnSpuriousLossDetected(packetNumber protocol.PacketNumber, packetReordering protocol.PacketNumber)
}

// PacketReorderingThresholdProvider is implemented by congestion controllers
// that want to override the default RFC 9002 packet reordering threshold.
type PacketReorderingThresholdProvider interface {
	GetPacketReorderThreshold() protocol.PacketNumber
}

// PTOHandler is implemented by congestion controllers that want an explicit
// QUIC PTO signal with live inflight.
type PTOHandler interface {
	OnPTO(bytesInFlight protocol.ByteCount)
}

// ConnectionMigrationHandler is implemented by custom congestion controllers
// that want to reset themselves in place on path migration.
type ConnectionMigrationHandler interface {
	OnConnectionMigration(initialMaxDatagramSize protocol.ByteCount)
}
