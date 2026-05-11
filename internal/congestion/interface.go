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

// ECNCongestionHandler is implemented by congestion controllers that take ownership
// of the ECN congestion decision. When implemented, quic-go forwards raw cumulative
// ECN counters from the ACK frame and skips its own congestion signal. Forwarding is
// gated on ECN path validation having succeeded.
type ECNCongestionHandler interface {
	OnECNCongestion(
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

// PTOHandler is implemented by congestion controllers that want an explicit
// QUIC PTO signal with live inflight.
type PTOHandler interface {
	OnPTO(bytesInFlight protocol.ByteCount)
}

// ConnectionMigrationHandler is implemented by custom congestion controllers
// that want to reset themselves in place on path migration.
//
// If not implemented, the controller is replaced with a fresh NewReno instance
// on path migration. RFC 9000 §9.4 mandates that the congestion controller MUST
// be reset to initial values when a new path is confirmed. Implement this hook
// if your algorithm manages its own state reset rather than being replaced.
type ConnectionMigrationHandler interface {
	OnConnectionMigration(initialMaxDatagramSize protocol.ByteCount)
}
