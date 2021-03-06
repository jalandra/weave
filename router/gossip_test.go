package router

import (
	"testing"
	"time"

	wt "github.com/weaveworks/weave/testing"
)

// TODO test gossip unicast; atm we only test topology gossip, which
// does not employ unicast.

type mockChannelConnection struct {
	RemoteConnection
	dest *Router
}

// Construct a "passive" Router, i.e. without any goroutines, except
// for Routes and GossipSenders.
func NewTestRouter(name string) *Router {
	peerName, _ := PeerNameFromString(name)
	router := NewRouter(Config{}, peerName, "")
	// need to create a dummy channel otherwise tests hang on nil
	// channels when the Router invoked ConnectionMaker.Refresh
	router.ConnectionMaker.actionChan = make(chan ConnectionMakerAction, ChannelSize)
	router.Routes.Start()
	return router
}

func (conn *mockChannelConnection) SendProtocolMsg(protocolMsg ProtocolMsg) {
	if err := conn.dest.handleGossip(protocolMsg.tag, protocolMsg.msg); err != nil {
		panic(err)
	}
}

func sendPendingGossip(routers ...*Router) {
	// Loop until all routers report they didn't send anything
	for sentSomething := true; sentSomething; {
		sentSomething = false
		for _, router := range routers {
			sentSomething = router.sendPendingGossip() || sentSomething
		}
	}
}

func (router *Router) AddTestChannelConnection(r *Router) {
	fromName := router.Ourself.Peer.Name
	toName := r.Ourself.Peer.Name

	fromPeer := NewPeer(fromName, "", router.Ourself.Peer.UID, 0)
	toPeer := NewPeer(toName, "", r.Ourself.Peer.UID, 0)

	r.Peers.FetchWithDefault(fromPeer)    // Has side-effect of incrementing refcount
	router.Peers.FetchWithDefault(toPeer) //

	conn := &mockChannelConnection{RemoteConnection{router.Ourself.Peer, toPeer, "", false, true}, r}
	router.Ourself.handleAddConnection(conn)
	router.Ourself.handleConnectionEstablished(conn)
}

func (router *Router) DeleteTestChannelConnection(r *Router) {
	fromName := router.Ourself.Peer.Name
	toName := r.Ourself.Peer.Name

	fromPeer, _ := r.Peers.Fetch(fromName)
	toPeer, _ := router.Peers.Fetch(toName)

	r.Peers.Dereference(fromPeer)
	router.Peers.Dereference(toPeer)

	conn, _ := router.Ourself.ConnectionTo(toName)
	router.Ourself.handleDeleteConnection(conn)
}

func TestGossipTopology(t *testing.T) {
	wt.RunWithTimeout(t, 5*time.Second, func() {
		implTestGossipTopology(t)
	})
}

// Create a Peer representing the receiver router, with connections to
// the routers supplied as arguments, carrying across all UID and
// version information.
func (router *Router) tp(routers ...*Router) *Peer {
	peer := NewPeer(router.Ourself.Peer.Name, "", router.Ourself.Peer.UID, 0)
	connections := make(map[PeerName]Connection)
	for _, r := range routers {
		p := NewPeer(r.Ourself.Peer.Name, "", r.Ourself.Peer.UID, r.Ourself.Peer.version)
		connections[r.Ourself.Peer.Name] = newMockConnection(peer, p)
	}
	peer.version = router.Ourself.Peer.version
	peer.connections = connections
	return peer
}

// Check that the topology of router matches the peers and all of their connections
func checkTopology(t *testing.T, router *Router, wantedPeers ...*Peer) {
	router.Peers.RLock()
	checkTopologyPeers(t, true, router.Peers.allPeers(), wantedPeers...)
	router.Peers.RUnlock()
}

func implTestGossipTopology(t *testing.T) {
	// Create some peers that will talk to each other
	r1 := NewTestRouter("01:00:00:01:00:00")
	r2 := NewTestRouter("02:00:00:02:00:00")
	r3 := NewTestRouter("03:00:00:03:00:00")

	// Check state when they have no connections
	checkTopology(t, r1, r1.tp())
	checkTopology(t, r2, r2.tp())

	// Now try adding some connections
	r1.AddTestChannelConnection(r2)
	sendPendingGossip(r1, r2)
	checkTopology(t, r1, r1.tp(r2), r2.tp())
	checkTopology(t, r2, r1.tp(r2), r2.tp())
	r2.AddTestChannelConnection(r1)
	sendPendingGossip(r1, r2)
	checkTopology(t, r1, r1.tp(r2), r2.tp(r1))
	checkTopology(t, r2, r1.tp(r2), r2.tp(r1))

	// Currently, the connection from 2 to 3 is one-way only
	r2.AddTestChannelConnection(r3)
	sendPendingGossip(r1, r2, r3)
	checkTopology(t, r1, r1.tp(r2), r2.tp(r1, r3), r3.tp())
	checkTopology(t, r2, r1.tp(r2), r2.tp(r1, r3), r3.tp())
	// When r2 gossiped to r3, 1 was unreachable from r3 so it got removed from the
	// list of peers, but remains referenced in the connection from 1 to 3.
	checkTopology(t, r3, r2.tp(r1, r3), r3.tp())

	// Add a connection from 3 to 1 and now r1 is reachable.
	r3.AddTestChannelConnection(r1)
	sendPendingGossip(r1, r2, r3)
	checkTopology(t, r1, r1.tp(r2), r2.tp(r1, r3), r3.tp(r1))
	checkTopology(t, r2, r1.tp(r2), r2.tp(r1, r3), r3.tp(r1))
	checkTopology(t, r3, r1.tp(), r2.tp(r1, r3), r3.tp(r1))

	r1.AddTestChannelConnection(r3)
	sendPendingGossip(r1, r2, r3)
	checkTopology(t, r1, r1.tp(r2, r3), r2.tp(r1, r3), r3.tp(r1))
	checkTopology(t, r2, r1.tp(r2, r3), r2.tp(r1, r3), r3.tp(r1))
	checkTopology(t, r3, r1.tp(r2, r3), r2.tp(r1, r3), r3.tp(r1))

	// Drop the connection from 2 to 3
	r2.DeleteTestChannelConnection(r3)
	sendPendingGossip(r1, r2, r3)
	checkTopology(t, r1, r1.tp(r2, r3), r2.tp(r1), r3.tp(r1))
	checkTopology(t, r2, r1.tp(r2, r3), r2.tp(r1))
	checkTopology(t, r3, r1.tp(r2, r3), r2.tp(r1), r3.tp(r1))

	// Drop the connection from 1 to 3
	r1.DeleteTestChannelConnection(r3)
	sendPendingGossip(r1, r2, r3)
	checkTopology(t, r1, r1.tp(r2), r2.tp(r1), r3.tp(r1))

	checkTopology(t, r1, r1.tp(r2), r2.tp(r1), r3.tp(r1))
	checkTopology(t, r2, r1.tp(r2), r2.tp(r1))
	// r3 still thinks r1 has a connection to it
	checkTopology(t, r3, r1.tp(r2, r3), r2.tp(r1), r3.tp(r1))

	// On a timer, r3 will gossip to r1
	r3.SendAllGossip()
	sendPendingGossip(r1, r2, r3)
	checkTopology(t, r1, r1.tp(r2), r2.tp(r1), r3.tp(r1))
}
