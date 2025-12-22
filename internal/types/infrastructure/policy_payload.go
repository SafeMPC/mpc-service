package infrastructure

import (
	"context"

	"github.com/go-openapi/errors"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"
)

// SetSigningPolicyPayload 定义
type SetSigningPolicyPayload struct {
	PolicyType    string           `json:"policy_type"`
	MinSignatures int32            `json:"min_signatures"`
	AdminAuths    []AdminAuthToken `json:"admin_auths"`
}

// Validate validates SetSigningPolicyPayload
func (m *SetSigningPolicyPayload) Validate(formats strfmt.Registry) error {
	var res []error

	if err := validate.RequiredString("policy_type", "body", m.PolicyType); err != nil {
		res = append(res, err)
	}

	if err := validate.Enum("policy_type", "body", m.PolicyType, []interface{}{"single", "team"}); err != nil {
		res = append(res, err)
	}

	if err := validate.MinimumInt("min_signatures", "body", int64(m.MinSignatures), 1, false); err != nil {
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
func (m *SetSigningPolicyPayload) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// SigningPolicyResponse 定义
type SigningPolicyResponse struct {
	Success       bool   `json:"success"`
	Message       string `json:"message"`
	KeyID         string `json:"key_id"`
	PolicyType    string `json:"policy_type"`
	MinSignatures int32  `json:"min_signatures"`
}

// Validate validates SigningPolicyResponse
func (m *SigningPolicyResponse) Validate(formats strfmt.Registry) error {
	return nil
}

// ContextValidate validates this payload based on context it is used
func (m *SigningPolicyResponse) ContextValidate(ctx context.Context, formats strfmt.Registry) error {
	return nil
}

// MarshalBinary interface implementation
func (m *SigningPolicyResponse) MarshalBinary() ([]byte, error) {
	if m == nil {
		return nil, nil
	}
	return swag.WriteJSON(m)
}

// UnmarshalBinary interface implementation
func (m *SigningPolicyResponse) UnmarshalBinary(b []byte) error {
	var res SigningPolicyResponse
	if err := swag.ReadJSON(b, &res); err != nil {
		return err
	}
	*m = res
	return nil
}
