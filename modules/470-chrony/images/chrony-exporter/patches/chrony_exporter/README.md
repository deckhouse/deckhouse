## patches

### 001-go-mod.patch

Bumps golang.org/x/crypto from v0.55.0 to v0.56.0 for CVE-2026-56855 and
CVE-2026-78662. x/crypto v0.56.0 requires go 1.26.0, so the go directive moves
from 1.25.0 to 1.26.0 and the image needs a Go 1.26+ builder.
