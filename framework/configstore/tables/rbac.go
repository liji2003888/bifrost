package tables

import "time"

// TableRbacRole represents a role in the RBAC system
type TableRbacRole struct {
	ID          string    `gorm:"primaryKey;type:varchar(255)" json:"id"`
	Name        string    `gorm:"type:varchar(255);not null;uniqueIndex" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	IsDefault   bool      `gorm:"not null;default:false" json:"is_default"`
	IsSystem    bool      `gorm:"not null;default:false" json:"is_system"` // admin, developer, viewer are system roles
	CreatedAt   time.Time `gorm:"index;not null" json:"created_at"`
	UpdatedAt   time.Time `gorm:"index;not null" json:"updated_at"`

	// Preloaded permissions (not stored directly in this table)
	Permissions []TableRbacPermission `gorm:"foreignKey:RoleID;constraint:OnDelete:CASCADE" json:"permissions,omitempty"`
}

func (TableRbacRole) TableName() string { return "rbac_roles" }

// TableRbacPermission represents a permission assignment for a role
type TableRbacPermission struct {
	ID        uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	RoleID    string `gorm:"type:varchar(255);not null;index;uniqueIndex:idx_rbac_perm" json:"role_id"`
	Resource  string `gorm:"type:varchar(255);not null;uniqueIndex:idx_rbac_perm" json:"resource"`
	Operation string `gorm:"type:varchar(50);not null;uniqueIndex:idx_rbac_perm" json:"operation"`
}

func (TableRbacPermission) TableName() string { return "rbac_permissions" }

// TableRbacUserRole maps users to roles
type TableRbacUserRole struct {
	ID     uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID string `gorm:"type:varchar(255);not null;uniqueIndex:idx_rbac_user_role" json:"user_id"`
	RoleID string `gorm:"type:varchar(255);not null;uniqueIndex:idx_rbac_user_role" json:"role_id"`
}

func (TableRbacUserRole) TableName() string { return "rbac_user_roles" }
