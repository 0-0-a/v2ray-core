package kcp

import (
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"v2ray.com/core/common/log"
	"v2ray.com/core/common/predicate"
	"v2ray.com/core/transport/internet"
	"v2ray.com/core/transport/internet/internal"
)

var (
	ErrIOTimeout        = errors.New("Read/Write timeout")
	ErrClosedListener   = errors.New("Listener closed.")
	ErrClosedConnection = errors.New("Connection closed.")
)

type State int32

func (this State) Is(states ...State) bool {
	for _, state := range states {
		if this == state {
			return true
		}
	}
	return false
}

const (
	StateActive          State = 0
	StateReadyToClose    State = 1
	StatePeerClosed      State = 2
	StateTerminating     State = 3
	StatePeerTerminating State = 4
	StateTerminated      State = 5
)

const (
	headerSize uint32 = 2
)

func nowMillisec() int64 {
	now := time.Now()
	return now.Unix()*1000 + int64(now.Nanosecond()/1000000)
}

type RoundTripInfo struct {
	sync.RWMutex
	variation        uint32
	srtt             uint32
	rto              uint32
	minRtt           uint32
	updatedTimestamp uint32
}

func (this *RoundTripInfo) UpdatePeerRTO(rto uint32, current uint32) {
	this.Lock()
	defer this.Unlock()

	if current-this.updatedTimestamp < 3000 {
		return
	}

	this.updatedTimestamp = current
	this.rto = rto
}

func (this *RoundTripInfo) Update(rtt uint32, current uint32) {
	if rtt > 0x7FFFFFFF {
		return
	}
	this.Lock()
	defer this.Unlock()

	// https://tools.ietf.org/html/rfc6298
	if this.srtt == 0 {
		this.srtt = rtt
		this.variation = rtt / 2
	} else {
		delta := rtt - this.srtt
		if this.srtt > rtt {
			delta = this.srtt - rtt
		}
		this.variation = (3*this.variation + delta) / 4
		this.srtt = (7*this.srtt + rtt) / 8
		if this.srtt < this.minRtt {
			this.srtt = this.minRtt
		}
	}
	var rto uint32
	if this.minRtt < 4*this.variation {
		rto = this.srtt + 4*this.variation
	} else {
		rto = this.srtt + this.variation
	}

	if rto > 10000 {
		rto = 10000
	}
	this.rto = rto * 5 / 4
	this.updatedTimestamp = current
}

func (this *RoundTripInfo) Timeout() uint32 {
	this.RLock()
	defer this.RUnlock()

	return this.rto
}

func (this *RoundTripInfo) SmoothedTime() uint32 {
	this.RLock()
	defer this.RUnlock()

	return this.srtt
}

type Updater struct {
	interval        time.Duration
	shouldContinue  predicate.Predicate
	shouldTerminate predicate.Predicate
	updateFunc      func()
	notifier        chan bool
}

func NewUpdater(interval uint32, shouldContinue predicate.Predicate, shouldTerminate predicate.Predicate, updateFunc func()) *Updater {
	u := &Updater{
		interval:        time.Duration(interval) * time.Millisecond,
		shouldContinue:  shouldContinue,
		shouldTerminate: shouldTerminate,
		updateFunc:      updateFunc,
		notifier:        make(chan bool, 1),
	}
	go u.Run()
	return u
}

func (this *Updater) WakeUp() {
	select {
	case this.notifier <- true:
	default:
	}
}

func (this *Updater) Run() {
	for <-this.notifier {
		if this.shouldTerminate() {
			return
		}
		for this.shouldContinue() {
			this.updateFunc()
			time.Sleep(this.interval)
		}
	}
}

type SystemConnection interface {
	net.Conn
	Id() internal.ConnectionId
	Reset(internet.Authenticator, func([]byte))
}

// Connection is a KCP connection over UDP.
type Connection struct {
	conn           SystemConnection
	connRecycler   internal.ConnectionRecyler
	block          internet.Authenticator
	rd             time.Time
	wd             time.Time // write deadline
	since          int64
	dataInputCond  *sync.Cond
	dataOutputCond *sync.Cond
	Config         *Config

	conv             uint16
	state            State
	stateBeginTime   uint32
	lastIncomingTime uint32
	lastPingTime     uint32

	mss       uint32
	roundTrip *RoundTripInfo

	receivingWorker *ReceivingWorker
	sendingWorker   *SendingWorker

	output *BufferedSegmentWriter

	dataUpdater *Updater
	pingUpdater *Updater

	reusable bool
}

// NewConnection create a new KCP connection between local and remote.
func NewConnection(conv uint16, sysConn SystemConnection, recycler internal.ConnectionRecyler, block internet.Authenticator, config *Config) *Connection {
	log.Info("KCP|Connection: creating connection ", conv)

	authWriter := &AuthenticationWriter{
		Authenticator: block,
		Writer:        sysConn,
		Config:        config,
	}

	conn := &Connection{
		conv:           conv,
		conn:           sysConn,
		connRecycler:   recycler,
		block:          block,
		since:          nowMillisec(),
		dataInputCond:  sync.NewCond(new(sync.Mutex)),
		dataOutputCond: sync.NewCond(new(sync.Mutex)),
		Config:         config,
		output:         NewSegmentWriter(authWriter),
		mss:            authWriter.Mtu() - DataSegmentOverhead,
		roundTrip: &RoundTripInfo{
			rto:    100,
			minRtt: config.Tti.GetValue(),
		},
	}
	sysConn.Reset(block, conn.Input)

	conn.receivingWorker = NewReceivingWorker(conn)
	conn.sendingWorker = NewSendingWorker(conn)

	isTerminating := func() bool {
		return conn.State().Is(StateTerminating, StateTerminated)
	}
	isTerminated := func() bool {
		return conn.State() == StateTerminated
	}
	conn.dataUpdater = NewUpdater(
		config.Tti.GetValue(),
		predicate.Not(isTerminating).And(predicate.Any(conn.sendingWorker.UpdateNecessary, conn.receivingWorker.UpdateNecessary)),
		isTerminating,
		conn.updateTask)
	conn.pingUpdater = NewUpdater(
		5000, // 5 seconds
		predicate.Not(isTerminated),
		isTerminated,
		conn.updateTask)
	conn.pingUpdater.WakeUp()

	return conn
}

func (this *Connection) Elapsed() uint32 {
	return uint32(nowMillisec() - this.since)
}

// Read implements the Conn Read method.
func (this *Connection) Read(b []byte) (int, error) {
	if this == nil {
		return 0, io.EOF
	}

	for {
		if this.State().Is(StateReadyToClose, StateTerminating, StateTerminated) {
			return 0, io.EOF
		}
		nBytes := this.receivingWorker.Read(b)
		if nBytes > 0 {
			return nBytes, nil
		}

		if this.State() == StatePeerTerminating {
			return 0, io.EOF
		}

		var timer *time.Timer
		if !this.rd.IsZero() {
			duration := this.rd.Sub(time.Now())
			if duration <= 0 {
				return 0, ErrIOTimeout
			}
			timer = time.AfterFunc(duration, this.dataInputCond.Signal)
		}
		this.dataInputCond.L.Lock()
		this.dataInputCond.Wait()
		this.dataInputCond.L.Unlock()
		if timer != nil {
			timer.Stop()
		}
		if !this.rd.IsZero() && this.rd.Before(time.Now()) {
			return 0, ErrIOTimeout
		}
	}
}

// Write implements the Conn Write method.
func (this *Connection) Write(b []byte) (int, error) {
	totalWritten := 0

	for {
		if this == nil || this.State() != StateActive {
			return totalWritten, io.ErrClosedPipe
		}

		nBytes := this.sendingWorker.Push(b[totalWritten:])
		this.dataUpdater.WakeUp()
		if nBytes > 0 {
			totalWritten += nBytes
			if totalWritten == len(b) {
				return totalWritten, nil
			}
		}

		var timer *time.Timer
		if !this.wd.IsZero() {
			duration := this.wd.Sub(time.Now())
			if duration <= 0 {
				return totalWritten, ErrIOTimeout
			}
			timer = time.AfterFunc(duration, this.dataOutputCond.Signal)
		}
		this.dataOutputCond.L.Lock()
		this.dataOutputCond.Wait()
		this.dataOutputCond.L.Unlock()

		if timer != nil {
			timer.Stop()
		}

		if !this.wd.IsZero() && this.wd.Before(time.Now()) {
			return totalWritten, ErrIOTimeout
		}
	}
}

func (this *Connection) SetState(state State) {
	current := this.Elapsed()
	atomic.StoreInt32((*int32)(&this.state), int32(state))
	atomic.StoreUint32(&this.stateBeginTime, current)
	log.Debug("KCP|Connection: #", this.conv, " entering state ", state, " at ", current)

	switch state {
	case StateReadyToClose:
		this.receivingWorker.CloseRead()
	case StatePeerClosed:
		this.sendingWorker.CloseWrite()
	case StateTerminating:
		this.receivingWorker.CloseRead()
		this.sendingWorker.CloseWrite()
		this.pingUpdater.interval = time.Second
	case StatePeerTerminating:
		this.sendingWorker.CloseWrite()
		this.pingUpdater.interval = time.Second
	case StateTerminated:
		this.receivingWorker.CloseRead()
		this.sendingWorker.CloseWrite()
		this.pingUpdater.interval = time.Second
		this.dataUpdater.WakeUp()
		this.pingUpdater.WakeUp()
		go this.Terminate()
	}
}

// Close closes the connection.
func (this *Connection) Close() error {
	if this == nil {
		return ErrClosedConnection
	}

	this.dataInputCond.Broadcast()
	this.dataOutputCond.Broadcast()

	state := this.State()
	if state.Is(StateReadyToClose, StateTerminating, StateTerminated) {
		return ErrClosedConnection
	}
	log.Info("KCP|Connection: Closing connection to ", this.conn.RemoteAddr())

	if state == StateActive {
		this.SetState(StateReadyToClose)
	}
	if state == StatePeerClosed {
		this.SetState(StateTerminating)
	}
	if state == StatePeerTerminating {
		this.SetState(StateTerminated)
	}

	return nil
}

// LocalAddr returns the local network address. The Addr returned is shared by all invocations of LocalAddr, so do not modify it.
func (this *Connection) LocalAddr() net.Addr {
	if this == nil {
		return nil
	}
	return this.conn.LocalAddr()
}

// RemoteAddr returns the remote network address. The Addr returned is shared by all invocations of RemoteAddr, so do not modify it.
func (this *Connection) RemoteAddr() net.Addr {
	if this == nil {
		return nil
	}
	return this.conn.RemoteAddr()
}

// SetDeadline sets the deadline associated with the listener. A zero time value disables the deadline.
func (this *Connection) SetDeadline(t time.Time) error {
	if err := this.SetReadDeadline(t); err != nil {
		return err
	}
	if err := this.SetWriteDeadline(t); err != nil {
		return err
	}
	return nil
}

// SetReadDeadline implements the Conn SetReadDeadline method.
func (this *Connection) SetReadDeadline(t time.Time) error {
	if this == nil || this.State() != StateActive {
		return ErrClosedConnection
	}
	this.rd = t
	return nil
}

// SetWriteDeadline implements the Conn SetWriteDeadline method.
func (this *Connection) SetWriteDeadline(t time.Time) error {
	if this == nil || this.State() != StateActive {
		return ErrClosedConnection
	}
	this.wd = t
	return nil
}

// kcp update, input loop
func (this *Connection) updateTask() {
	this.flush()
}

func (this *Connection) Reusable() bool {
	return false
}

func (this *Connection) SetReusable(b bool) {}

func (this *Connection) Terminate() {
	if this == nil {
		return
	}
	log.Info("KCP|Connection: Terminating connection to ", this.RemoteAddr())

	//this.SetState(StateTerminated)
	this.dataInputCond.Broadcast()
	this.dataOutputCond.Broadcast()
	if this.Config.ConnectionReuse.IsEnabled() && this.reusable {
		this.connRecycler.Put(this.conn.Id(), this.conn)
	} else {
		this.conn.Close()
	}
	this.sendingWorker.Release()
	this.receivingWorker.Release()
}

func (this *Connection) HandleOption(opt SegmentOption) {
	if (opt & SegmentOptionClose) == SegmentOptionClose {
		this.OnPeerClosed()
	}
}

func (this *Connection) OnPeerClosed() {
	state := this.State()
	if state == StateReadyToClose {
		this.SetState(StateTerminating)
	}
	if state == StateActive {
		this.SetState(StatePeerClosed)
	}
}

// Input when you received a low level packet (eg. UDP packet), call it
func (this *Connection) Input(data []byte) {
	current := this.Elapsed()
	atomic.StoreUint32(&this.lastIncomingTime, current)

	var seg Segment
	for {
		seg, data = ReadSegment(data)
		if seg == nil {
			break
		}
		if seg.Conversation() != this.conv {
			return
		}

		switch seg := seg.(type) {
		case *DataSegment:
			this.HandleOption(seg.Option)
			this.receivingWorker.ProcessSegment(seg)
			this.dataInputCond.Signal()
			this.dataUpdater.WakeUp()
		case *AckSegment:
			this.HandleOption(seg.Option)
			this.sendingWorker.ProcessSegment(current, seg, this.roundTrip.Timeout())
			this.dataOutputCond.Signal()
			this.dataUpdater.WakeUp()
		case *CmdOnlySegment:
			this.HandleOption(seg.Option)
			if seg.Command == CommandTerminate {
				state := this.State()
				if state == StateActive ||
					state == StatePeerClosed {
					this.SetState(StatePeerTerminating)
				} else if state == StateReadyToClose {
					this.SetState(StateTerminating)
				} else if state == StateTerminating {
					this.SetState(StateTerminated)
				}
			}
			this.sendingWorker.ProcessReceivingNext(seg.ReceivinNext)
			this.receivingWorker.ProcessSendingNext(seg.SendingNext)
			this.roundTrip.UpdatePeerRTO(seg.PeerRTO, current)
			seg.Release()
		default:
		}
	}
}

func (this *Connection) flush() {
	current := this.Elapsed()

	if this.State() == StateTerminated {
		return
	}
	if this.State() == StateActive && current-atomic.LoadUint32(&this.lastIncomingTime) >= 30000 {
		this.Close()
	}
	if this.State() == StateReadyToClose && this.sendingWorker.IsEmpty() {
		this.SetState(StateTerminating)
	}

	if this.State() == StateTerminating {
		log.Debug("KCP|Connection: #", this.conv, " sending terminating cmd.")
		this.Ping(current, CommandTerminate)
		this.output.Flush()

		if current-atomic.LoadUint32(&this.stateBeginTime) > 8000 {
			this.SetState(StateTerminated)
		}
		return
	}
	if this.State() == StatePeerTerminating && current-atomic.LoadUint32(&this.stateBeginTime) > 4000 {
		this.SetState(StateTerminating)
	}

	if this.State() == StateReadyToClose && current-atomic.LoadUint32(&this.stateBeginTime) > 15000 {
		this.SetState(StateTerminating)
	}

	// flush acknowledges
	this.receivingWorker.Flush(current)
	this.sendingWorker.Flush(current)

	if current-atomic.LoadUint32(&this.lastPingTime) >= 3000 {
		this.Ping(current, CommandPing)
	}

	// flash remain segments
	this.output.Flush()
}

func (this *Connection) State() State {
	return State(atomic.LoadInt32((*int32)(&this.state)))
}

func (this *Connection) Ping(current uint32, cmd Command) {
	seg := NewCmdOnlySegment()
	seg.Conv = this.conv
	seg.Command = cmd
	seg.ReceivinNext = this.receivingWorker.nextNumber
	seg.SendingNext = this.sendingWorker.firstUnacknowledged
	seg.PeerRTO = this.roundTrip.Timeout()
	if this.State() == StateReadyToClose {
		seg.Option = SegmentOptionClose
	}
	this.output.Write(seg)
	atomic.StoreUint32(&this.lastPingTime, current)
	seg.Release()
}
