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

// Other roles are allowed and handled as opaque strings, only naming convention is validated.
//+kubebuilder:validation:Pattern:=`^[a-z_]+$`
type MasterCellRole string

const (
	// Handling cypress tree or sub-tree (portal). Default role for both primary and secondary cells.
	MasterCellRoleCypressNodeHost        MasterCellRole = "cypress_node_host"
	// Handling transactions. Default role for primary cell.
	MasterCellRoleTransactionCoordinator MasterCellRole = "transaction_coordinator"
	// Handling chunk metadata. Default role for secondary cells and for primary if there is no secondary cells.
	MasterCellRoleChunkHost              MasterCellRole = "chunk_host"
)

func GetMasterCellRoles(roles []MasterCellRole, isPrimary, isMulticell bool) []MasterCellRole {
	switch {
	case len(roles) > 0:
		return roles
	case isPrimary && isMulticell:
		// NOTE: In multicell primary master have no chunk host role by default.
		// But it may have this role if cluster were upgraded from non-multicell,
		// in this case roles of master cell must be listed explicitly.
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
