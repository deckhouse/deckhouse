# Copyright 2022 Flant JSC
# Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE.

# On node bootstrap always update repository info
if [ "$FIRST_BASHIBLE_RUN" == "yes" ]; then
  bb-yum-update
fi
