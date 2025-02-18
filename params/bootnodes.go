// Copyright 2015 The go-ethereum Authors
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

// MainnetBootnodes are the enode URLs of the P2P bootstrap nodes running on
// the main Ethereum network.
var MainnetBootnodes = []string{
	"enode://0ec8f957266eb79c56fc422c28643119a0b7b9771f0cd1a3dc91dc1b865b29e25e3856703bd8fe040556c443cea2ff13fd5bf432adfa3445a3366e0eb9ae063d@13.52.121.63:36000",
	"enode://a8cabd43c04d550a8cde3786dd1e0937f8239f43e57cefca04da10b2d614a7aed4224ffa23cfb1c1e02327e3afeb1496a9fe231b2d5a1ecfc165efd89457b838@13.52.17.225:36000",
	"enode://714a6ba37ece30e436d35572e2d98ddcc222ff5043f7ed520c942a2c18408afe7f60e29eba9a979533c5bbad26dd365654368afecd6c6e6376d95292e1be0dd8@13.52.193.35:36000",
	"enode://fa84e30d55809f9d15a2dbc21c58f0f732a67e3afae2ddb530e9c871be04bff49c50f073beb5ce27ba62b23ebd778424424c2ee5cf059148b465d536caa7c20a@13.57.173.244:36000",
}

// TestnetBootnodes are the enode URLs of the P2P bootstrap nodes running on the
// Ropsten test network.
var TestnetBootnodes = []string{}

// RinkebyBootnodes are the enode URLs of the P2P bootstrap nodes running on the
// Rinkeby test network.
var RinkebyBootnodes = []string{}

// DiscoveryV5Bootnodes are the enode URLs of the P2P bootstrap nodes for the
// experimental RLPx v5 topic-discovery network.
var DiscoveryV5Bootnodes = []string{}
