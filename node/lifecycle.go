// Copyright 2020 The go-ethereum Authors
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

package node

// Lifecycle encompasses the behavior of services that can be started and stopped
// on the node. Lifecycle management is delegated to the node, but it is the
// responsibility of the service-specific package to configure and register the
// service on the node using the `RegisterLifecycle` method.
//
// Start 在网络层起来之后调用，用来起后台循环。Stop 要阻塞到这些循环结束。
// 普通模式下 engine API 不是 Lifecycle。会注册成后台服务的是 --dev 的模拟信标和 --beacon.api 的 blsync。
type Lifecycle interface {
	// Start is called after all services have been constructed and the networking
	// layer was also initialized to spawn any goroutines required by the service.
	Start() error

	// Stop terminates all goroutines belonging to the service, blocking until they
	// are all terminated.
	Stop() error
}
