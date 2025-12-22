package infrastructure

import (
	"context"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"
)

// AdminAuthToken 定义
type AdminAuthToken struct {
	ReqID             string `json:"req_id"`
	CredentialID      string `json:"credential_id"`
	PasskeySignature  []byte `json:"passkey_signature"`
	AuthenticatorData []byte `json:"authenticator_data"`
	ClientDataJSON    []byte `json:"client_data_json"`
}

// Validate validates AdminAuthToken
func (m *AdminAuthToken) Validate(formats strfmt.Registry) error {
	return nil
}

// AddWalletMemberPayload 定义
type AddWalletMemberPayload struct {
	CredentialID string           `json:"credential_id"`
	Role         string           `json:"role"`
	AdminAuths   []AdminAuthToken `json:"admin_auths"`
}

// Validate validates AddWalletMemberPayload
func (m *AddWalletMemberPayload) Validate(formats strfmt.Registry) error {
	var res []error

	if err := validate.RequiredString("credential_id", "body", m.CredentialID); err != nil {
		res = append(res, err)
	}

	if err := validate.RequiredString("role", "body", m.Role); err != nil {
		res = append(res, err)
	}

	if len(m.AdminAuths) == 0 {
		res = append(res, errors.Required("admin_auths", "body", m.AdminAuths))
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

// ContextValidate validates this payload based on context it is used
func (m *AddWalletMemberPayload) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// RemoveWalletMemberPayload 定义
type RemoveWalletMemberPayload struct {
	CredentialID string           `json:"credential_id"`
	AdminAuths   []AdminAuthToken `json:"admin_auths"`
}

// Validate validates RemoveWalletMemberPayload
func (m *RemoveWalletMemberPayload) Validate(formats strfmt.Registry) error {
	var res []error

	if err := validate.RequiredString("credential_id", "body", m.CredentialID); err != nil {
		res = append(res, err)
	}

	if len(m.AdminAuths) == 0 {
		res = append(res, errors.Required("admin_auths", "body", m.AdminAuths))
	}

	if len(res) > 0 {
		return errors.CompositeValidationError(res...)
	}
	return nil
}

// ContextValidate validates this payload based on context it is used
func (m *RemoveWalletMemberPayload) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// WalletMemberResponse 定义
type WalletMemberResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// Validate validates WalletMemberResponse
func (m *WalletMemberResponse) Validate(formats strfmt.Registry) error {
	return nil
}

// ContextValidate validates this payload based on context it is used
func (m *WalletMemberResponse) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (m *WalletMemberResponse) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *WalletMemberResponse) UnmarshalBinary(b []byte) error {
	var res WalletMemberResponse
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
