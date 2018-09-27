package username

import (
	"github.com/iov-one/weave"
	"github.com/iov-one/weave/errors"
	"github.com/iov-one/weave/orm"
	"github.com/iov-one/weave/x"
	"github.com/iov-one/weave/x/nft"
)

const (
	createUsernameCost = 0
)

// RegisterRoutes will instantiate and register all handlers in this package
func RegisterRoutes(r weave.Registry, auth x.Authenticator, issuer weave.Address) {
	bucket := NewBucket()
	r.Handle(pathIssueTokenMsg, NewIssueHandler(auth, issuer, bucket))
	r.Handle(pathAddAddressMsg, NewAddChainAddressHandler(auth, issuer, bucket))
	r.Handle(pathRemoveAddressMsg, NewRemoveChainAddressHandler(auth, issuer, bucket))
}

// RegisterQuery will register this bucket as "/nft/usernames"
func RegisterQuery(qr weave.QueryRouter) {
	NewBucket().Register("nft/usernames", qr)
}

type tokenHandler struct {
	auth   x.Authenticator
	issuer weave.Address
	bucket Bucket
}

type IssueHandler struct {
	tokenHandler
}

func NewIssueHandler(auth x.Authenticator, issuer weave.Address, bucket Bucket) *IssueHandler {
	return &IssueHandler{tokenHandler{auth: auth, issuer: issuer, bucket: bucket}}
}

func (h IssueHandler) Check(ctx weave.Context, store weave.KVStore, tx weave.Tx) (weave.CheckResult, error) {
	var res weave.CheckResult
	if _, err := h.validate(ctx, tx); err != nil {
		return res, err
	}
	res.GasAllocated += createUsernameCost
	return res, nil
}

func (h IssueHandler) Deliver(ctx weave.Context, store weave.KVStore, tx weave.Tx) (weave.DeliverResult, error) {
	var res weave.DeliverResult
	msg, err := h.validate(ctx, tx)
	if err != nil {
		return res, err
	}
	// persist the data
	o, err := h.bucket.Create(store, weave.Address(msg.Owner), msg.Id, msg.Details.Addresses)
	if err != nil {
		return res, err
	}

	//ha, err := AsUsername(o)
	//if err != nil {
	//	return res, err
	//}
	//for _, a := range msg.Approvals {
	//	for _, approval := range a.Approvals {
	//		// todo: apply approval validation rules:
	//		//if err := ha.Approvals().Set(a.Action, approval.ToAccount, approval.Options); err != nil {
	//		//	return res, err
	//		//}
	//	}
	//}
	return res, h.bucket.Save(store, o)
}

func (h IssueHandler) validate(ctx weave.Context, tx weave.Tx) (*IssueTokenMsg, error) {
	rmsg, err := tx.GetMsg()
	if err != nil {
		return nil, err
	}
	msg, ok := rmsg.(*IssueTokenMsg)
	if !ok {
		return nil, errors.ErrUnknownTxType(rmsg)
	}
	if err := msg.Validate(); err != nil {
		return nil, err
	}
	// check permissions
	if h.issuer != nil {
		if !h.auth.HasAddress(ctx, h.issuer) {
			return nil, errors.ErrUnauthorized()
		}
	} else {
		if !h.auth.HasAddress(ctx, msg.Owner) {
			return nil, errors.ErrUnauthorized()
		}
	}
	return msg, nil
}

type AddChainAddressHandler struct {
	tokenHandler
}

func NewAddChainAddressHandler(auth x.Authenticator, issuer weave.Address, bucket Bucket) *AddChainAddressHandler {
	return &AddChainAddressHandler{tokenHandler{auth: auth, issuer: issuer, bucket: bucket}}
}

func (h AddChainAddressHandler) Check(ctx weave.Context, store weave.KVStore, tx weave.Tx) (weave.CheckResult, error) {
	var res weave.CheckResult
	if _, err := h.validate(ctx, tx); err != nil {
		return res, err
	}
	res.GasAllocated += createUsernameCost
	return res, nil
}

func (h AddChainAddressHandler) Deliver(ctx weave.Context, store weave.KVStore, tx weave.Tx) (weave.DeliverResult, error) {
	var res weave.DeliverResult
	msg, err := h.validate(ctx, tx)
	if err != nil {
		return res, err
	}
	o, t, err := loadToken(h.tokenHandler, store, msg.GetId())
	if err != nil {
		return res, err
	}
	actor := findActor(h.tokenHandler, ctx, t)
	if actor == nil {
		return res, errors.ErrUnauthorized()
	}
	allKeys := append(t.GetChainAddresses(), ChainAddress{msg.GetChainID(), msg.GetAddress()})
	if err := t.SetChainAddresses(actor, allKeys); err != nil {
		return res, err
	}
	return res, h.bucket.Save(store, o)
}

func (h *AddChainAddressHandler) validate(ctx weave.Context, tx weave.Tx) (*AddChainAddressMsg, error) {
	rmsg, err := tx.GetMsg()
	if err != nil {
		return nil, err
	}
	msg, ok := rmsg.(*AddChainAddressMsg)
	if !ok {
		return nil, errors.ErrUnknownTxType(rmsg)
	}
	if err := msg.Validate(); err != nil {
		return nil, err
	}
	return msg, nil
}

type RemoveChainAddressHandler struct {
	tokenHandler
}

func NewRemoveChainAddressHandler(auth x.Authenticator, issuer weave.Address, bucket Bucket) *RemoveChainAddressHandler {
	return &RemoveChainAddressHandler{tokenHandler{auth: auth, issuer: issuer, bucket: bucket}}
}

func (h RemoveChainAddressHandler) Check(ctx weave.Context, store weave.KVStore, tx weave.Tx) (weave.CheckResult, error) {
	var res weave.CheckResult
	if _, err := h.validate(ctx, tx); err != nil {
		return res, err
	}
	res.GasAllocated += createUsernameCost
	return res, nil
}

func (h RemoveChainAddressHandler) Deliver(ctx weave.Context, store weave.KVStore, tx weave.Tx) (weave.DeliverResult, error) {
	var res weave.DeliverResult
	msg, err := h.validate(ctx, tx)
	if err != nil {
		return res, err
	}
	o, t, err := loadToken(h.tokenHandler, store, msg.GetId())
	if err != nil {
		return res, err
	}
	actor := findActor(h.tokenHandler, ctx, t)
	if actor == nil {
		return res, errors.ErrUnauthorized()
	}
	if len(t.GetChainAddresses()) == 0 {
		return res, nft.ErrInvalidEntry()
	}
	obsoleteAddress := ChainAddress{msg.GetChainID(), msg.GetAddress()}
	newAddresses := make([]ChainAddress, 0, len(t.GetChainAddresses()))
	for _, v := range t.GetChainAddresses() {
		if !v.Equals(obsoleteAddress) {
			newAddresses = append(newAddresses, v)
		}
	}
	if len(newAddresses) == len(t.GetChainAddresses()) {
		return res, nft.ErrInvalidEntry()
	}
	if err := t.SetChainAddresses(actor, newAddresses); err != nil {
		return res, err
	}
	return res, h.bucket.Save(store, o)
}

func (h *RemoveChainAddressHandler) validate(ctx weave.Context, tx weave.Tx) (*RemoveChainAddressMsg, error) {
	rmsg, err := tx.GetMsg()
	if err != nil {
		return nil, err
	}
	msg, ok := rmsg.(*RemoveChainAddressMsg)
	if !ok {
		return nil, errors.ErrUnknownTxType(rmsg)
	}
	if err := msg.Validate(); err != nil {
		return nil, err
	}
	return msg, nil
}

func loadToken(h tokenHandler, store weave.KVStore, id []byte) (orm.Object, Token, error) {
	o, err := h.bucket.Get(store, id)
	switch {
	case err != nil:
		return nil, nil, err
	case o == nil:
		return nil, nil, nft.ErrUnknownID()
	}
	t, e := AsUsername(o)
	return o, t, e
}

func findActor(h tokenHandler, ctx weave.Context, t Token) weave.Address {
	if h.auth.HasAddress(ctx, t.OwnerAddress()) {
		return t.OwnerAddress()
	} else {
		signers := x.GetAddresses(ctx, h.auth)
		if len(signers) == 1 {
			return signers[0]
		} else {
			for _, signers := range signers {
				_ = signers
				// todo: if update approval for signer exists in t then set actor
				// else
				return nil
				//return res, errors.ErrUnauthorized()
			}
		}
	}
	return nil
}

type AddRemoveApprovalHandler struct {
	tokenHandler
}

func (h *AddRemoveApprovalHandler) Check(ctx weave.Context, store weave.KVStore, tx weave.Tx) (weave.CheckResult, error) {
	var res weave.CheckResult
	if _, err := h.validate(ctx, tx); err != nil {
		return res, err
	}
	res.GasAllocated += createUsernameCost
	return res, nil
}

func (h *AddRemoveApprovalHandler) Deliver(ctx weave.Context, store weave.KVStore, tx weave.Tx) (weave.DeliverResult, error) {
	var res weave.DeliverResult
	msg, err := h.validate(ctx, tx)
	if err != nil {
		return res, err
	}

	o, t, err := loadToken(h.tokenHandler, store, msg.GetId())
	if err != nil {
		return res, err
	}

	actor := findActor(h.tokenHandler, ctx, t)
	if actor == nil {
		return res, errors.ErrUnauthorized()
	}

	switch v := interface{}(msg).(type) {
	case *AddApprovalMsg:
		base := v.Base

		err = t.Approvals().Grant(base.Action, base.Address, base.Options, 0)
		if err != nil {
			return res, err
		}
	case *RemoveApprovalMsg:
		base := v.Base

		err = t.Approvals().Revoke(base.Action, base.Address)
		if err != nil {
			return res, err
		}
	}

	return res, h.bucket.Save(store, o)
}

func (h *AddRemoveApprovalHandler) validate(ctx weave.Context, tx weave.Tx) (nft.Identified, error) {
	msg, err := tx.GetMsg()
	if err != nil {
		return nil, err
	}

	imsg := interface{}(msg)
	switch v := imsg.(type) {
	case *AddApprovalMsg, *RemoveApprovalMsg:
		if err := imsg.(x.Validater).Validate(); err != nil {
			return nil, err
		}
		return v.(nft.Identified), nil
	default:
		return nil, errors.ErrUnknownTxType(msg)
	}
}