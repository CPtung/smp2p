package signaling

type OnDescReceived func(desc []byte)
type OnCandidateReceived func(candidate []byte)

type Signal interface {
	Connect() error
	Disconnect()
	SendICESdp(string, []byte) error
	SendICECandidate(string, string) error
	AddICESdpListener(OnDescReceived)
	AddICECandidateListener(OnCandidateReceived)
}
