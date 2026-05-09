package models

import (
	"time"

	"gorm.io/gorm"
)

type ContentType string

const (
	ContentTypeVideo ContentType = "video"
	ContentTypeImage ContentType = "image"
	ContentTypeText  ContentType = "text"
)

type AuditStatus string

const (
	AuditStatusPending  AuditStatus = "pending"
	AuditStatusApproved AuditStatus = "approved"
	AuditStatusRejected AuditStatus = "rejected"
)

type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Username  string         `gorm:"uniqueIndex;size:50;not null" json:"username"`
	Password  string         `gorm:"size:255;not null" json:"-"`
	IsAdmin   bool           `gorm:"default:false" json:"is_admin"`
	IsBanned  bool           `gorm:"default:false" json:"is_banned"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type Content struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	Title       string         `gorm:"size:200;not null" json:"title"`
	Type        ContentType    `gorm:"size:20;not null;index" json:"type"`
	Content     string         `gorm:"type:text" json:"content,omitempty"`
	FilePath    string         `gorm:"size:500" json:"file_path,omitempty"`
	FileSize    int64          `json:"file_size,omitempty"`
	ThumbPath   string         `gorm:"size:500" json:"thumb_path,omitempty"`
	UserID      uint           `gorm:"not null;index" json:"user_id"`
	User        User           `json:"user,omitempty"`
	BigTagID    *uint          `gorm:"index" json:"big_tag_id,omitempty"`
	SmallTagID  *uint          `gorm:"index" json:"small_tag_id,omitempty"`
	Tags        []string       `gorm:"type:text;serializer:json" json:"tags,omitempty"`
	AuditStatus AuditStatus    `gorm:"size:20;default:pending;index" json:"audit_status"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

type Comment struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	ContentID uint           `gorm:"not null;index" json:"content_id"`
	Content   Content        `json:"content,omitempty"`
	UserID    uint           `gorm:"not null;index" json:"user_id"`
	User      User           `json:"user,omitempty"`
	Text      string         `gorm:"type:text;not null" json:"text"`
	ParentID  *uint          `gorm:"index" json:"parent_id,omitempty"`
	Parent    *Comment       `gorm:"foreignKey:ParentID;references:ID" json:"parent,omitempty"`
	Replies   []Comment      `gorm:"foreignKey:ParentID;references:ID" json:"replies,omitempty"`
	IsBanned  bool           `gorm:"default:false;index" json:"is_banned"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type CommentReport struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CommentID uint           `gorm:"not null;index" json:"comment_id"`
	Comment   Comment        `json:"comment,omitempty"`
	UserID    uint           `gorm:"not null;index" json:"user_id"`
	User      User           `json:"user,omitempty"`
	Reason    string         `gorm:"size:255" json:"reason"`
	Handled   bool           `gorm:"default:false" json:"handled"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type AuditLog struct {
	ID        uint        `gorm:"primaryKey" json:"id"`
	ContentID uint        `gorm:"not null;index" json:"content_id"`
	Content   Content     `json:"content,omitempty"`
	AdminID   uint        `gorm:"not null;index" json:"admin_id"`
	Admin     User        `json:"admin,omitempty"`
	Status    AuditStatus `gorm:"size:20;not null" json:"status"`
	Remark    string      `gorm:"type:text" json:"remark,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
}
