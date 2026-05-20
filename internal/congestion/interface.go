package congestion

import (
	"time"

	"github.com/quic-go/quic-go/internal/monotime"
	"github.com/quic-go/quic-go/internal/protocol"
)

// A SendAlgorithm performs congestion control
type SendAlgorithm interface {
	TimeUntilSend(bytesInFlight protocol.ByteCount) monotime.Time
	HasPacingBudget(now monotime.Time) bool
	OnPacketSent(sentTime monotime.Time, bytesInFlight protocol.ByteCount, packetNumber protocol.PacketNumber, bytes protocol.ByteCount, isRetransmittable bool)
	CanSend(bytesInFlight protocol.ByteCount) bool
	MaybeExitSlowStart()
	OnPacketAcked(number protocol.PacketNumber, ackedBytes protocol.ByteCount, priorInFlight protocol.ByteCount, eventTime monotime.Time)
	OnCongestionEvent(number protocol.PacketNumber, lostBytes protocol.ByteCount, priorInFlight protocol.ByteCount)
	OnRetransmissionTimeout(packetsRetransmitted bool)
	SetMaxDatagramSize(protocol.ByteCount)
}

// A SendAlgorithmWithDebugInfos is a SendAlgorithm that exposes some debug infos
type SendAlgorithmWithDebugInfos interface {
	SendAlgorithm
	InSlowStart() bool
	InRecovery() bool
	GetCongestionWindow() protocol.ByteCount
}

// CongestionController is the public interface that external CC algorithms must implement.
// All timestamps use stdlib time.Time. Defined here (not in the top-level quic package)
// so internal/ackhandler can reference it without a circular import.
type CongestionController interface {
	TimeUntilSend(bytesInFlight protocol.ByteCount) time.Time
	HasPacingBudget(now time.Time) bool
	CanSend(bytesInFlight protocol.ByteCount) bool
	OnPacketSent(sentTime time.Time, bytesInFlight protocol.ByteCount, packetNumber protocol.PacketNumber, bytes protocol.ByteCount, isRetransmittable bool)
	OnPacketAcked(number protocol.PacketNumber, ackedBytes protocol.ByteCount, priorInFlight protocol.ByteCount, eventTime time.Time)
	OnCongestionEvent(number protocol.PacketNumber, lostBytes protocol.ByteCount, priorInFlight protocol.ByteCount)
	SetMaxDatagramSize(protocol.ByteCount)
	MaybeExitSlowStart()
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
