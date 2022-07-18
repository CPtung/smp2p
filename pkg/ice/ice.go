package ice

type CandidateInfo struct {
	Source    string `json:"source"`
	Candidate string `json:"candidate"`
}

type SdpInfo struct {
	Source string `json:"source"`
	Sdp    string `json:"sdp"`
}

type Offer interface {
	AddCandidateListener()
	AddSdpListener()
	Close()
}

type Answer interface {
	AddCandidateListener()
	AddSdpListener()
	Close()
}

type Desc struct {
	Name string
}
