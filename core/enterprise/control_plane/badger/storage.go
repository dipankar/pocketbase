package badger

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	badger "github.com/dgraph-io/badger/v4"
	"github.com/pocketbase/pocketbase/core/enterprise"
)

// Storage wraps BadgerDB for control plane metadata storage
type Storage struct {
	db          *badger.DB
	diskManager *DiskManager
}

// NewStorage creates a new BadgerDB storage instance
func NewStorage(dataDir string) (*Storage, error) {
	dbPath := filepath.Join(dataDir, "badger")

	opts := badger.DefaultOptions(dbPath)
	opts.Logger = nil // Disable BadgerDB's verbose logging

	// Configure BadgerDB for production use
	opts.NumVersionsToKeep = 1           // Keep only latest version
	opts.CompactL0OnClose = true         // Compact on close
	opts.ValueLogFileSize = 64 << 20     // 64 MB value log files
	opts.NumLevelZeroTables = 5          // Trigger compaction after 5 L0 tables
	opts.NumLevelZeroTablesStall = 10    // Stall writes after 10 L0 tables
	opts.ValueLogMaxEntries = 500000     // Max entries per value log file

	db, err := badger.Open(opts)
	if err != nil {
		return nil, fmt.Errorf("failed to open BadgerDB: %w", err)
	}

	// Initialize disk manager with conservative limits
	diskConfig := &DiskConfig{
		MaxDiskUsageBytes:    10 * 1024 * 1024 * 1024, // 10 GB
		WarningThresholdPct:  80.0,
		CriticalThresholdPct: 95.0,
		GCInterval:           5 * time.Minute,
		GCDiscardRatio:       0.5,
		CompactionInterval:   1 * time.Hour,
		CheckInterval:        1 * time.Minute,
	}

	diskManager := NewDiskManager(db, diskConfig)
	diskManager.Start()

	return &Storage{
		db:          db,
		diskManager: diskManager,
	}, nil
}

// Close closes the BadgerDB instance
func (s *Storage) Close() error {
	if s.diskManager != nil {
		s.diskManager.Stop()
	}
	return s.db.Close()
}

// GetDiskManager returns the disk manager for monitoring
func (s *Storage) GetDiskManager() *DiskManager {
	return s.diskManager
}

// Key prefixes for different data types
const (
	keyPrefixTenant            = "tenant:"
	keyPrefixTenantDomain      = "tenant_domain:"
	keyPrefixUser              = "user:"
	keyPrefixUserEmail         = "user_email:"
	keyPrefixNode              = "node:"
	keyPrefixPlacement         = "placement:"
	keyPrefixQuotaRequest      = "quota_req:"
	keyPrefixAdminToken        = "admin_token:"
	keyPrefixActivity          = "activity:"            // Tenant activity tracking
	keyPrefixAccessPattern     = "access_pattern:"      // Tenant access patterns
	keyPrefixVerificationToken = "verification_token:" // Email verification tokens
)

// Tenant operations

func (s *Storage) GetTenant(tenantID string) (*enterprise.Tenant, error) {
	var tenant enterprise.Tenant

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(keyPrefixTenant + tenantID))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return enterprise.ErrTenantNotFound
			}
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &tenant)
		})
	})

	if err != nil {
		return nil, err
	}

	return &tenant, nil
}

func (s *Storage) GetTenantByDomain(domain string) (*enterprise.Tenant, error) {
	var tenantID string

	// First lookup: domain -> tenant ID
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(keyPrefixTenantDomain + domain))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return enterprise.ErrTenantNotFound
			}
			return err
		}

		return item.Value(func(val []byte) error {
			tenantID = string(val)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	// Second lookup: tenant ID -> tenant
	return s.GetTenant(tenantID)
}

func (s *Storage) CreateTenant(tenant *enterprise.Tenant) error {
	tenantJSON, err := json.Marshal(tenant)
	if err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		// Check if tenant already exists
		_, err := txn.Get([]byte(keyPrefixTenant + tenant.ID))
		if err == nil {
			return enterprise.ErrTenantAlreadyExists
		}

		// Check if domain already exists
		_, err = txn.Get([]byte(keyPrefixTenantDomain + tenant.Domain))
		if err == nil {
			return fmt.Errorf("domain %s already in use", tenant.Domain)
		}

		// Save tenant
		if err := txn.Set([]byte(keyPrefixTenant+tenant.ID), tenantJSON); err != nil {
			return err
		}

		// Save domain -> tenant ID mapping
		return txn.Set([]byte(keyPrefixTenantDomain+tenant.Domain), []byte(tenant.ID))
	})
}

func (s *Storage) UpdateTenant(tenant *enterprise.Tenant) error {
	tenantJSON, err := json.Marshal(tenant)
	if err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		// Verify tenant exists
		_, err := txn.Get([]byte(keyPrefixTenant + tenant.ID))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return enterprise.ErrTenantNotFound
			}
			return err
		}

		return txn.Set([]byte(keyPrefixTenant+tenant.ID), tenantJSON)
	})
}

func (s *Storage) UpdateTenantStatus(tenantID string, status enterprise.TenantStatus) error {
	return s.db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(keyPrefixTenant + tenantID))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return enterprise.ErrTenantNotFound
			}
			return err
		}

		var tenant enterprise.Tenant
		err = item.Value(func(val []byte) error {
			return json.Unmarshal(val, &tenant)
		})
		if err != nil {
			return err
		}

		tenant.Status = status
		tenantJSON, err := json.Marshal(&tenant)
		if err != nil {
			return err
		}

		return txn.Set([]byte(keyPrefixTenant+tenant.ID), tenantJSON)
	})
}

func (s *Storage) DeleteTenant(tenantID string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		// Get tenant first to get domain
		item, err := txn.Get([]byte(keyPrefixTenant + tenantID))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return enterprise.ErrTenantNotFound
			}
			return err
		}

		var tenant enterprise.Tenant
		err = item.Value(func(val []byte) error {
			return json.Unmarshal(val, &tenant)
		})
		if err != nil {
			return err
		}

		// Delete domain mapping
		if err := txn.Delete([]byte(keyPrefixTenantDomain + tenant.Domain)); err != nil {
			return err
		}

		// Delete tenant
		return txn.Delete([]byte(keyPrefixTenant + tenantID))
	})
}

// User operations

func (s *Storage) GetUser(userID string) (*enterprise.ClusterUser, error) {
	var user enterprise.ClusterUser

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(keyPrefixUser + userID))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return enterprise.ErrUserNotFound
			}
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &user)
		})
	})

	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (s *Storage) GetUserByEmail(email string) (*enterprise.ClusterUser, error) {
	var userID string

	// First lookup: email -> user ID
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(keyPrefixUserEmail + email))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return enterprise.ErrUserNotFound
			}
			return err
		}

		return item.Value(func(val []byte) error {
			userID = string(val)
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	// Second lookup: user ID -> user
	return s.GetUser(userID)
}

func (s *Storage) CreateUser(user *enterprise.ClusterUser) error {
	userJSON, err := json.Marshal(user)
	if err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		// Check if user already exists
		_, err := txn.Get([]byte(keyPrefixUser + user.ID))
		if err == nil {
			return enterprise.ErrUserAlreadyExists
		}

		// Check if email already exists
		_, err = txn.Get([]byte(keyPrefixUserEmail + user.Email))
		if err == nil {
			return fmt.Errorf("email %s already registered", user.Email)
		}

		// Save user
		if err := txn.Set([]byte(keyPrefixUser+user.ID), userJSON); err != nil {
			return err
		}

		// Save email -> user ID mapping
		return txn.Set([]byte(keyPrefixUserEmail+user.Email), []byte(user.ID))
	})
}

func (s *Storage) UpdateUser(user *enterprise.ClusterUser) error {
	userJSON, err := json.Marshal(user)
	if err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		// Verify user exists
		_, err := txn.Get([]byte(keyPrefixUser + user.ID))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return enterprise.ErrUserNotFound
			}
			return err
		}

		return txn.Set([]byte(keyPrefixUser+user.ID), userJSON)
	})
}

func (s *Storage) CountUserTenants(userID string) (int, error) {
	count := 0

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(keyPrefixTenant)

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var tenant enterprise.Tenant
				if err := json.Unmarshal(val, &tenant); err != nil {
					return err
				}

				if tenant.OwnerUserID == userID && tenant.Status != enterprise.TenantStatusDeleted {
					count++
				}

				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	return count, err
}

// ListUsers returns all cluster users with optional pagination
func (s *Storage) ListUsers(limit, offset int) ([]*enterprise.ClusterUser, int, error) {
	users := make([]*enterprise.ClusterUser, 0)
	totalCount := 0

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(keyPrefixUser)

		it := txn.NewIterator(opts)
		defer it.Close()

		currentIndex := 0
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()

			// Count all users for total
			totalCount++

			// Apply offset
			if currentIndex < offset {
				currentIndex++
				continue
			}

			// Apply limit
			if limit > 0 && len(users) >= limit {
				continue
			}

			err := item.Value(func(val []byte) error {
				var user enterprise.ClusterUser
				if err := json.Unmarshal(val, &user); err != nil {
					return err
				}
				users = append(users, &user)
				return nil
			})

			if err != nil {
				return err
			}

			currentIndex++
		}

		return nil
	})

	return users, totalCount, err
}

// ListTenants returns all tenants with optional pagination and filtering
func (s *Storage) ListTenants(limit, offset int, ownerUserID string) ([]*enterprise.Tenant, int, error) {
	tenants := make([]*enterprise.Tenant, 0)
	totalCount := 0

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(keyPrefixTenant)

		it := txn.NewIterator(opts)
		defer it.Close()

		currentIndex := 0
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				var tenant enterprise.Tenant
				if err := json.Unmarshal(val, &tenant); err != nil {
					return err
				}

				// Skip deleted tenants
				if tenant.Status == enterprise.TenantStatusDeleted {
					return nil
				}

				// Apply owner filter if specified
				if ownerUserID != "" && tenant.OwnerUserID != ownerUserID {
					return nil
				}

				// Count matching tenants
				totalCount++

				// Apply offset
				if currentIndex < offset {
					currentIndex++
					return nil
				}

				// Apply limit
				if limit > 0 && len(tenants) >= limit {
					return nil
				}

				tenants = append(tenants, &tenant)
				currentIndex++
				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	return tenants, totalCount, err
}

// Node operations

func (s *Storage) SaveNode(node *enterprise.NodeInfo) error {
	nodeJSON, err := json.Marshal(node)
	if err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(keyPrefixNode+node.ID), nodeJSON)
	})
}

func (s *Storage) GetNode(nodeID string) (*enterprise.NodeInfo, error) {
	var node enterprise.NodeInfo

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(keyPrefixNode + nodeID))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return enterprise.ErrNodeNotFound
			}
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &node)
		})
	})

	if err != nil {
		return nil, err
	}

	return &node, nil
}

func (s *Storage) ListNodes() ([]*enterprise.NodeInfo, error) {
	nodes := make([]*enterprise.NodeInfo, 0)

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(keyPrefixNode)

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var node enterprise.NodeInfo
				if err := json.Unmarshal(val, &node); err != nil {
					return err
				}
				nodes = append(nodes, &node)
				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	return nodes, err
}

// ListTenantsByNode returns all tenants assigned to a specific node
func (s *Storage) ListTenantsByNode(nodeID string) ([]*enterprise.Tenant, error) {
	tenants := make([]*enterprise.Tenant, 0)

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(keyPrefixTenant)

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()

			err := item.Value(func(val []byte) error {
				var tenant enterprise.Tenant
				if err := json.Unmarshal(val, &tenant); err != nil {
					return err
				}

				// Skip deleted tenants
				if tenant.Status == enterprise.TenantStatusDeleted {
					return nil
				}

				// Filter by assigned node
				if tenant.AssignedNodeID == nodeID {
					tenants = append(tenants, &tenant)
				}

				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	return tenants, err
}

// Placement operations

func (s *Storage) SavePlacement(placement *enterprise.PlacementDecision) error {
	placementJSON, err := json.Marshal(placement)
	if err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(keyPrefixPlacement+placement.TenantID), placementJSON)
	})
}

func (s *Storage) GetPlacement(tenantID string) (*enterprise.PlacementDecision, error) {
	var placement enterprise.PlacementDecision

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(keyPrefixPlacement + tenantID))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return enterprise.ErrTenantNotAssigned
			}
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &placement)
		})
	})

	if err != nil {
		return nil, err
	}

	return &placement, nil
}

// Activity tracking operations

func (s *Storage) SaveActivity(activity *enterprise.TenantActivity) error {
	activityJSON, err := json.Marshal(activity)
	if err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(keyPrefixActivity+activity.TenantID), activityJSON)
	})
}

func (s *Storage) GetActivity(tenantID string) (*enterprise.TenantActivity, error) {
	var activity enterprise.TenantActivity

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(keyPrefixActivity + tenantID))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				// Return default activity if not found
				now := time.Now()
				activity = enterprise.TenantActivity{
					TenantID:    tenantID,
					LastAccess:  now,
					AccessCount: 0,
					StorageTier: enterprise.StorageTierHot,
					Created:     now,
					Updated:     now,
				}
				return nil
			}
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &activity)
		})
	})

	if err != nil {
		return nil, err
	}

	return &activity, nil
}

func (s *Storage) RecordTenantAccess(tenantID string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		var activity enterprise.TenantActivity

		// Get existing activity
		item, err := txn.Get([]byte(keyPrefixActivity + tenantID))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				// Create new activity record
				now := time.Now()
				activity = enterprise.TenantActivity{
					TenantID:        tenantID,
					LastAccess:      now,
					AccessCount:     1,
					StorageTier:     enterprise.StorageTierHot,
					RequestsLast24h: 1,
					RequestsLast7d:  1,
					Created:         now,
					Updated:         now,
				}
			} else {
				return err
			}
		} else {
			err = item.Value(func(val []byte) error {
				return json.Unmarshal(val, &activity)
			})
			if err != nil {
				return err
			}

			// Update activity
			activity.LastAccess = time.Now()
			activity.AccessCount++
			activity.RequestsLast24h++
			activity.RequestsLast7d++
			activity.Updated = time.Now()
		}

		// Save updated activity
		activityJSON, err := json.Marshal(&activity)
		if err != nil {
			return err
		}

		return txn.Set([]byte(keyPrefixActivity+tenantID), activityJSON)
	})
}

// ListInactiveTenants returns tenants that haven't been accessed since the given time
func (s *Storage) ListInactiveTenants(since time.Time) ([]*enterprise.TenantActivity, error) {
	activities := make([]*enterprise.TenantActivity, 0)

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(keyPrefixActivity)

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var activity enterprise.TenantActivity
				if err := json.Unmarshal(val, &activity); err != nil {
					return err
				}

				// Only include tenants inactive since the given time
				if activity.LastAccess.Before(since) {
					activities = append(activities, &activity)
				}

				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	return activities, err
}

// ListActivitiesByTier returns all tenant activities for a given storage tier
func (s *Storage) ListActivitiesByTier(tier enterprise.StorageTier) ([]*enterprise.TenantActivity, error) {
	activities := make([]*enterprise.TenantActivity, 0)

	err := s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(keyPrefixActivity)

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				var activity enterprise.TenantActivity
				if err := json.Unmarshal(val, &activity); err != nil {
					return err
				}

				if activity.StorageTier == tier {
					activities = append(activities, &activity)
				}

				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	return activities, err
}

// Access pattern operations

func (s *Storage) SaveAccessPattern(pattern *enterprise.TenantAccessPattern) error {
	patternJSON, err := json.Marshal(pattern)
	if err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		return txn.Set([]byte(keyPrefixAccessPattern+pattern.TenantID), patternJSON)
	})
}

func (s *Storage) GetAccessPattern(tenantID string) (*enterprise.TenantAccessPattern, error) {
	var pattern enterprise.TenantAccessPattern

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(keyPrefixAccessPattern + tenantID))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				// Return empty pattern if not found
				now := time.Now()
				pattern = enterprise.TenantAccessPattern{
					TenantID:          tenantID,
					DayOfWeek:         []int{},
					HourOfDay:         []int{},
					PatternConfidence: 0.0,
					Created:           now,
					Updated:           now,
				}
				return nil
			}
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &pattern)
		})
	})

	if err != nil {
		return nil, err
	}

	return &pattern, nil
}

// Verification token operations

func (s *Storage) SaveVerificationToken(token *enterprise.VerificationToken) error {
	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return err
	}

	return s.db.Update(func(txn *badger.Txn) error {
		// Set token with TTL based on expiration
		entry := badger.NewEntry([]byte(keyPrefixVerificationToken+token.Token), tokenJSON).
			WithTTL(time.Until(token.Expires))
		return txn.SetEntry(entry)
	})
}

func (s *Storage) GetVerificationToken(token string) (*enterprise.VerificationToken, error) {
	var verificationToken enterprise.VerificationToken

	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(keyPrefixVerificationToken + token))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return fmt.Errorf("verification token not found or expired")
			}
			return err
		}

		return item.Value(func(val []byte) error {
			return json.Unmarshal(val, &verificationToken)
		})
	})

	if err != nil {
		return nil, err
	}

	// Double check expiration
	if time.Now().After(verificationToken.Expires) {
		return nil, fmt.Errorf("verification token expired")
	}

	// Check if already used
	if verificationToken.Used {
		return nil, fmt.Errorf("verification token already used")
	}

	return &verificationToken, nil
}

func (s *Storage) MarkVerificationTokenUsed(token string) error {
	return s.db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(keyPrefixVerificationToken + token))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return fmt.Errorf("verification token not found")
			}
			return err
		}

		var verificationToken enterprise.VerificationToken
		err = item.Value(func(val []byte) error {
			return json.Unmarshal(val, &verificationToken)
		})
		if err != nil {
			return err
		}

		verificationToken.Used = true

		tokenJSON, err := json.Marshal(&verificationToken)
		if err != nil {
			return err
		}

		return txn.Set([]byte(keyPrefixVerificationToken+token), tokenJSON)
	})
}

// UseVerificationTokenAtomically validates and marks a token as used in a single atomic operation
// This prevents double-use of tokens due to race conditions
func (s *Storage) UseVerificationTokenAtomically(token string) (*enterprise.VerificationToken, error) {
	var verificationToken enterprise.VerificationToken

	err := s.db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(keyPrefixVerificationToken + token))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return fmt.Errorf("verification token not found or expired")
			}
			return err
		}

		err = item.Value(func(val []byte) error {
			return json.Unmarshal(val, &verificationToken)
		})
		if err != nil {
			return err
		}

		// Check expiration
		if time.Now().After(verificationToken.Expires) {
			return fmt.Errorf("verification token expired")
		}

		// Check if already used
		if verificationToken.Used {
			return fmt.Errorf("verification token already used")
		}

		// Mark as used in the same transaction
		verificationToken.Used = true

		tokenJSON, err := json.Marshal(&verificationToken)
		if err != nil {
			return err
		}

		return txn.Set([]byte(keyPrefixVerificationToken+token), tokenJSON)
	})

	if err != nil {
		return nil, err
	}

	return &verificationToken, nil
}

// ExportData exports all key-value pairs from BadgerDB
// The visitor function is called for each key-value pair
func (s *Storage) ExportData(visitor func(key, value []byte) error) error {
	return s.db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100 // Prefetch for better performance

		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()

			err := item.Value(func(val []byte) error {
				// Call visitor with copies of key and value
				keyCopy := append([]byte(nil), key...)
				valCopy := append([]byte(nil), val...)
				return visitor(keyCopy, valCopy)
			})

			if err != nil {
				return err
			}
		}

		return nil
	})
}

// ImportData imports key-value pairs into BadgerDB
// This clears existing data and restores from the provided entries
func (s *Storage) ImportData(entries []SnapshotEntry) error {
	return s.db.Update(func(txn *badger.Txn) error {
		// First, delete all existing keys
		// We need to iterate and collect keys to delete
		keysToDelete := make([][]byte, 0)

		opts := badger.DefaultIteratorOptions
		opts.PrefetchSize = 100

		it := txn.NewIterator(opts)
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.KeyCopy(nil)
			keysToDelete = append(keysToDelete, key)
		}
		it.Close()

		// Delete all keys
		for _, key := range keysToDelete {
			if err := txn.Delete(key); err != nil {
				return fmt.Errorf("failed to delete key during restore: %w", err)
			}
		}

		// Import new data
		for _, entry := range entries {
			if err := txn.Set(entry.Key, entry.Value); err != nil {
				return fmt.Errorf("failed to set key during restore: %w", err)
			}
		}

		return nil
	})
}

// SnapshotEntry represents a key-value pair for import/export
type SnapshotEntry struct {
	Key   []byte
	Value []byte
}
