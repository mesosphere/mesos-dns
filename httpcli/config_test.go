package httpcli_test

import (
	"testing"

	. "github.com/mesosphere/mesos-dns/httpcli"
	"github.com/mesosphere/mesos-dns/httpcli/basic"
	"github.com/mesosphere/mesos-dns/httpcli/iam"
)

func TestValidate(t *testing.T) {
	basic.Register()
	iam.Register()
	defer RegistryReset()

	for i, tc := range []struct {
		am           AuthMechanism
		cm           ConfigMap
		expectingErr bool
		expectedErr  error
	}{
		{am: "", cm: nil}, // sanity check, auth-none should not require any configuration
		{am: "123", cm: nil, expectingErr: true, expectedErr: ErrUnregisteredFactory},
		{am: AuthIAM, cm: nil, expectingErr: true, expectedErr: ErrMissingConfiguration},
		{am: AuthBasic, cm: nil, expectingErr: true, expectedErr: ErrMissingConfiguration},

		{ // IAM expects configuration
			am:           AuthIAM,
			cm:           ConfigMapOptions{iam.Configuration(iam.Config{})}.ToConfigMap(),
			expectingErr: true,
			expectedErr:  iam.ErrInvalidConfiguration,
		},
		{ // valid IAM configuration
			am: AuthIAM,
			cm: ConfigMapOptions{iam.Configuration(iam.Config{
				ID:            "foo",
				Secret:        "bar",
				LoginEndpoint: "blah",
			})}.ToConfigMap(),
		},
		{ // Basic expects configuration
			am:           AuthBasic,
			cm:           ConfigMapOptions{basic.Configuration(basic.Credentials{})}.ToConfigMap(),
			expectingErr: true,
			expectedErr:  basic.ErrInvalidConfiguration,
		},
		{ // valid Basic configuration
			am: AuthBasic,
			cm: ConfigMapOptions{basic.Configuration(basic.Credentials{
				Principal: "foo",
				Secret:    "bar",
			})}.ToConfigMap(),
		},
		{ // valid Basic configuration, principal only
			am: AuthBasic,
			cm: ConfigMapOptions{basic.Configuration(basic.Credentials{
				Principal: "foo",
			})}.ToConfigMap(),
		},
		{ // valid Basic configuration, secret only
			am: AuthBasic,
			cm: ConfigMapOptions{basic.Configuration(basic.Credentials{
				Secret: "bar",
			})}.ToConfigMap(),
		},
	} {
		err := Validate(tc.am, tc.cm)
		if tc.expectingErr && err == nil {
			t.Errorf("test case %d expected error but got none", i)
		}
		if !tc.expectingErr && err != nil {
			t.Errorf("test case %d unexpected error: %+v", i, err)
		}
		if tc.expectingErr && err != nil && tc.expectedErr != nil && tc.expectedErr != err {
			t.Errorf("test case %d expected error %v but got %v instead", i, tc.expectedErr, err)
		}
	}
}
