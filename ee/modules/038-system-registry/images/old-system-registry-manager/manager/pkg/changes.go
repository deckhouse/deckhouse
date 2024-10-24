/*
Copyright 2024 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package pkg

type NeedChangeFileBy struct {
	NeedChangeFileByExist             bool
	NeedChangeFileByCheckSum          *bool
	NeedChangeFileByDataInconsistency *bool
}

func (n *NeedChangeFileBy) NeedChange() bool {
	if n.NeedChangeFileByExist {
		return true
	}
	if n.NeedChangeFileByCheckSum != nil && *n.NeedChangeFileByCheckSum {
		return true
	}
	if n.NeedChangeFileByDataInconsistency != nil && *n.NeedChangeFileByDataInconsistency {
		return true
	}
	return false
}

func (n *NeedChangeFileBy) NeedCreate() bool {
	return n.NeedChangeFileByExist
}

func (n *NeedChangeFileBy) NeedUpdate() bool {
	if n.NeedChangeFileByCheckSum != nil && *n.NeedChangeFileByCheckSum {
		return true
	}
	if n.NeedChangeFileByDataInconsistency != nil && *n.NeedChangeFileByDataInconsistency {
		return true
	}
	return false
}
