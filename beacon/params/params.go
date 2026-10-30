// Copyright 2022 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package params

// EpochLength 是 32 个 slot，给轻客户端校验用。
// SyncPeriodLength 是 8192 个 slot，一个同步委员会服务这么久。
// 超多数是委员会的 2/3。签名人数不到这个数，更新不算近似最终。
const (
	EpochLength      = 32
	SyncPeriodLength = 8192

	BLSSignatureSize = 96
	BLSPubkeySize    = 48

	SyncCommitteeSize          = 512
	SyncCommitteeBitmaskSize   = SyncCommitteeSize / 8
	SyncCommitteeSupermajority = (SyncCommitteeSize*2 + 2) / 3
)

const (
	StateIndexGenesisTime              = 32
	StateIndexGenesisValidators        = 33
	StateIndexForkVersion              = 141
	StateIndexLatestHeader             = 36
	StateIndexBlockRoots               = 37
	StateIndexStateRoots               = 38
	StateIndexHistoricRoots            = 39
	StateIndexFinalBlockOld            = 105
	StateIndexFinalBlockElectra        = 169
	StateIndexSyncCommitteeOld         = 54
	StateIndexSyncCommitteeElectra     = 86
	StateIndexNextSyncCommitteeOld     = 55
	StateIndexNextSyncCommitteeElectra = 87
	StateIndexExecPayload              = 56
	StateIndexExecHead                 = 908

	BodyIndexExecPayload = 25
)

func StateIndexFinalBlock(forkName string) uint64 {
	switch forkName {
	case "bellatrix", "capella", "deneb":
		return StateIndexFinalBlockOld
	default:
		return StateIndexFinalBlockElectra
	}
}
func StateIndexSyncCommittee(forkName string) uint64 {
	switch forkName {
	case "bellatrix", "capella", "deneb":
		return StateIndexSyncCommitteeOld
	default:
		return StateIndexSyncCommitteeElectra
	}
}
func StateIndexNextSyncCommittee(forkName string) uint64 {
	switch forkName {
	case "bellatrix", "capella", "deneb":
		return StateIndexNextSyncCommitteeOld
	default:
		return StateIndexNextSyncCommitteeElectra
	}
}
