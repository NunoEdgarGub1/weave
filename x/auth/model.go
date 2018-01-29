package auth

import (
	"fmt"

	"github.com/confio/weave"
	"github.com/confio/weave/crypto"
)

//----------------- Model ------------------
//
// Model stores the persistent state and all domain logic
// associated with valid state and state transitions.
// As well as how to de/serialize it from the persistent store.
//
// If does not care about when to change state or who is authorized
// (that belongs in the controller), but rather with what possible
// states are valid.

//------------------ Serialization ------------------------

//---- Key

// UserKey is the primary key we use to distinguish users
// This should be []byte, in order to index with our KVStore.
// Any structure to these bytes should be defined by the constructor.
//
// Question: allow objects with a Marshal method???
type UserKey []byte

var userPrefix = []byte("user:")

// NewUserKey constructs the user key from a key hash,
// by appending a prefix.
func NewUserKey(addr weave.Address) UserKey {
	bz := append(userPrefix, addr...)
	return UserKey(bz)
}

//---- Data

// Validate must determine if this is a legal state
// (eg. all required fields set, sequence non-negative, etc.)
//
// Returns an explanation if the data is invalid
func (u UserData) Validate() error {
	if u.Sequence < 0 {
		// TODO: ErrInvalidSequence
		return fmt.Errorf("Sequence is negative")
	}
	if u.Sequence > 0 && u.PubKey == nil {
		// TODO: ErrInvalidSequence
		return fmt.Errorf("Positive Sequence must have a PubKey")
	}
	return nil
}

//------------------ High-Level ------------------------

// User is the actual object that we want to pass around in our code.
// It handles loading and saving the data to/from the persistent store.
// It also adds helpers to manipulate state.
//
// It may allow full access to manipulate all variables on the data,
// or limit it. It maintains a reference to the store it was loaded
// from, to know how to save itself.
type User struct {
	store weave.KVStore
	key   UserKey
	data  UserData
}

// GetUser loads this user if present, or returns nil if missing
func GetUser(store weave.KVStore, key UserKey) *User {
	bz := store.Get(key)
	if bz == nil {
		return nil
	}

	var data UserData
	weave.MustUnmarshal(&data, bz)

	return &User{
		store: store,
		key:   key,
		data:  data,
	}
}

// GetOrCreateUser loads this user if present,
// or initializes a new user with this key if not present.
func GetOrCreateUser(store weave.KVStore, key UserKey) *User {
	res := GetUser(store, key)
	if res == nil {
		res = &User{
			store: store,
			key:   key,
			data:  UserData{},
		}
	}
	return res
}

// Save writes the current user state to the backing store
// panics if invalid state
func (u *User) Save() {
	// TODO: MustValidate
	err := u.data.Validate()
	if err != nil {
		panic(err)
	}

	// TODO: MustMarshal
	value, err := u.data.Marshal()
	if err != nil {
		panic(err)
	}

	u.store.Set(u.key, value)
}

// Delete removes the current user id from the backing store
// panics if key is missing
// func (u *User) Delete() {
// 	u.store.Delete(u.key)
// }

// PubKey checks the current pubkey for this account
func (u User) PubKey() *crypto.PublicKey {
	return u.data.GetPubKey()
}

// HasPubKey returns true iff the pubkey has been set
func (u User) HasPubKey() bool {
	return u.data.GetPubKey() != nil
}

// Sequence checks the current sequence for this account
func (u User) Sequence() int64 {
	return u.data.Sequence
}

// CheckAndIncrementSequence checks if the current Sequence
// matches the expected value.
// If so, it will increase the sequence by one and return nil
// If not, it will not change the sequence, but return an error
func (u *User) CheckAndIncrementSequence(check int64) error {
	if u.data.Sequence != check {
		// TODO: ErrInvalidSequence
		return fmt.Errorf("Invalid sequence number %d (actual %d)",
			check, u.data.Sequence)
	}
	u.data.Sequence++
	return nil
}

// SetPubKey will try to set the PubKey or panic on an illegal operation.
// It is illegal to reset an already set key
// Otherwise, we don't control
// (although we could verify the hash, we leave that to the controller)
func (u *User) SetPubKey(pubKey *crypto.PublicKey) {
	if u.HasPubKey() {
		panic("Cannot change pubkey for a user")
	}
	u.data.PubKey = pubKey
}