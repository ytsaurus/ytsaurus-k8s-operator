package v1

// +kubebuilder:validation:Pattern:=`^[a-z_]+$`
type MasterCellRole string

const (
	MasterCellRoleCypressNodeHost        MasterCellRole = "cypress_node_host"
	MasterCellRoleTransactionCoordinator MasterCellRole = "transaction_coordinator"
	MasterCellRoleChunkHost              MasterCellRole = "chunk_host"
)

type MasterCellSpec struct {
	// Cell tag is an immutable identifier for particular cell of YTsaurus cluster.
	// Must be unique among all connected clusters to prevent object ids from colliding.
	//+kubebuilder:validation:Required
	//+kubebuilder:validation:Minimum:=1
	//+kubebuilder:validation:Maximum:=61440
	CellTag       uint16   `json:"cellTag"`
	HostAddresses []string `json:"hostAddresses,omitempty"`
	// Roles assigned to this master cell.
	//+listType=set
	Roles []MasterCellRole `json:"roles,omitempty"`
}

func (s MasterCellSpec) GetRoles(isPrimary, isMulticell bool) []MasterCellRole {
	switch {
	case len(s.Roles) > 0:
		return s.Roles
	case isPrimary && isMulticell:
		// NOTE: In multicell primary master have no chunk host role by default.
		// But it may have this role if cluster were upgraded from non-multicell.
		return []MasterCellRole{
			MasterCellRoleCypressNodeHost,
			MasterCellRoleTransactionCoordinator,
		}
	case isPrimary:
		return []MasterCellRole{
			MasterCellRoleCypressNodeHost,
			MasterCellRoleTransactionCoordinator,
			MasterCellRoleChunkHost,
		}
	default:
		return []MasterCellRole{
			MasterCellRoleCypressNodeHost,
			MasterCellRoleChunkHost,
		}
	}
}
