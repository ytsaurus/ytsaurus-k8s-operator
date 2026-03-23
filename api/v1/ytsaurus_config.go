package v1

type MasterConnectionSpec struct {
	// Cell tag is an immutable identifier for particular cell of YTsaurus cluster.
	// Must be unique among all connected clusters to prevent object ids from colliding.
	//+kubebuilder:validation:Required
	//+kubebuilder:validation:Minimum:=1
	//+kubebuilder:validation:Maximum:=61440
	CellTag uint16 `json:"cellTag"`
	//+listType=set
	HostAddresses []string `json:"hostAddresses,omitempty"`
}

type MasterCachesConnectionSpec struct {
	//+listType=set
	HostAddresses []string `json:"hostAddresses,omitempty"`
}
