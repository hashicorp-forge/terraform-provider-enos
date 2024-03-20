// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package remoteflight

import (
	"testing"

	"github.com/stretchr/testify/require"
)

var testOSReleaseHasID = `
NAME="Red Hat Enterprise Linux"
VERSION="9.1 (Plow)"
ID="rhel"
ID_LIKE="fedora"
VERSION_ID="9.1"
PLATFORM_ID="platform:el9"
PRETTY_NAME="Red Hat Enterprise Linux 9.1 (Plow)"
ANSI_COLOR="0;31"
LOGO="fedora-logo-icon"
CPE_NAME="cpe:/o:redhat:enterprise_linux:9::baseos"
HOME_URL="https://www.redhat.com/"
DOCUMENTATION_URL="https://access.redhat.com/documentation/red_hat_enterprise_linux/9/"
BUG_REPORT_URL="https://bugzilla.redhat.com/"

REDHAT_BUGZILLA_PRODUCT="Red Hat Enterprise Linux 9"
REDHAT_BUGZILLA_PRODUCT_VERSION=9.1
REDHAT_SUPPORT_PRODUCT="Red Hat Enterprise Linux"
REDHAT_SUPPORT_PRODUCT_VERSION="9.1"
`

var testOSReleaseNoID = `
NAME="Red Hat Enterprise Linux"
VERSION="9.1 (Plow)"
ID_LIKE="fedora"
VERSION_ID="9.1"
PLATFORM_ID="platform:el9"
PRETTY_NAME="Red Hat Enterprise Linux 9.1 (Plow)"
ANSI_COLOR="0;31"
LOGO="fedora-logo-icon"
CPE_NAME="cpe:/o:redhat:enterprise_linux:9::baseos"
HOME_URL="https://www.redhat.com/"
DOCUMENTATION_URL="https://access.redhat.com/documentation/red_hat_enterprise_linux/9/"
BUG_REPORT_URL="https://bugzilla.redhat.com/"

REDHAT_BUGZILLA_PRODUCT="Red Hat Enterprise Linux 9"
REDHAT_BUGZILLA_PRODUCT_VERSION=9.1
REDHAT_SUPPORT_PRODUCT="Red Hat Enterprise Linux"
REDHAT_SUPPORT_PRODUCT_VERSION="9.1"
`

func TestFindInOsRelease(t *testing.T) {
	t.Parallel()

	t.Run("has id", func(t *testing.T) {
		t.Parallel()
		id, err := findInOsRelease(testOSReleaseHasID, "ID")
		require.NoError(t, err)
		require.Equal(t, "rhel", id)
	})

	t.Run("no id", func(t *testing.T) {
		t.Parallel()
		_, err := findInOsRelease(testOSReleaseNoID, "ID")
		require.Error(t, err)
	})
}
