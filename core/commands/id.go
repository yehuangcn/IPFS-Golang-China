
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
package commands

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	core "github.com/ipfs/go-ipfs/core"
	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"

	kb "gx/ipfs/QmNPEgLVQZeHM7eH5hSfdsTUtgjB1RsndQRFgzHAFkmmC5/go-libp2p-kbucket"
	ic "gx/ipfs/QmNiJiXwWE3kRhZrC5ej3kSjWHm337pYfhjLGSCDNKJP2s/go-libp2p-crypto"
	pstore "gx/ipfs/QmPiemjiKBC9VA7vZF82m4x1oygtg2c2YVqag8PX7dN1BD/go-libp2p-peerstore"
	cmds "gx/ipfs/QmWGm4AbZEbnmdgVTza52MSNpEmBdFVqzmAysRbjrRyGbH/go-ipfs-cmds"
	"gx/ipfs/QmY5Grm8pJdiSSVsYxx4uNRgweY72EmYwuSDbRnbFok3iY/go-libp2p-peer"
	identify "gx/ipfs/QmYxivS34F2M2n44WQQnRHGAKS8aoRUxwGpi9wk4Cdn4Jf/go-libp2p/p2p/protocol/identify"
	"gx/ipfs/Qmde5VP1qUkyQXKCfmEUA7bP64V2HAptbJ7phuPp7jXWwg/go-ipfs-cmdkit"
)

const offlineIdErrorMessage = `'ipfs id' currently cannot query information on remote
peers without a running daemon; we are working to fix this.
In the meantime, if you want to query remote peers using 'ipfs id',
please run the daemon:

    ipfs daemon &
    ipfs id QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ
`

type IdOutput struct {
	ID              string
	PublicKey       string
	Addresses       []string
	AgentVersion    string
	ProtocolVersion string
}

const (
	formatOptionName = "format"
)

var IDCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Show ipfs node id info.",
		ShortDescription: `
Prints out information about the specified peer.
If no peer is specified, prints out information for local peers.

'ipfs id' supports the format option for output with the following keys:
<id> : The peers id.
<aver>: Agent version.
<pver>: Protocol version.
<pubkey>: Public key.
<addrs>: Addresses (newline delimited).

EXAMPLE:

    ipfs id Qmece2RkXhsKe5CRooNisBTh4SK119KrXXGmoK6V3kb8aH -f="<addrs>\n"
`,
	},
	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("peerid", false, false, "Peer.ID of node to look up."),
	},
	Options: []cmdkit.Option{
		cmdkit.StringOption(formatOptionName, "f", "Optional output format."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		var id peer.ID
		if len(req.Arguments) > 0 {
			var err error
			id, err = peer.IDB58Decode(req.Arguments[0])
			if err != nil {
				return fmt.Errorf("invalid peer id")
			}
		} else {
			id = n.Identity
		}

		if id == n.Identity {
			output, err := printSelf(n)
			if err != nil {
				return err
			}
			return cmds.EmitOnce(res, output)
		}

//TODO使用多态性而不是条件来处理脱机模式
		if !n.OnlineMode() {
			return errors.New(offlineIdErrorMessage)
		}

		p, err := n.Routing.FindPeer(req.Context, id)
		if err == kb.ErrLookupFailure {
			return errors.New(offlineIdErrorMessage)
		}
		if err != nil {
			return err
		}

		output, err := printPeer(n.Peerstore, p.ID)
		if err != nil {
			return err
		}
		return cmds.EmitOnce(res, output)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *IdOutput) error {
			format, found := req.Options[formatOptionName].(string)
			if found {
				output := format
				output = strings.Replace(output, "<id>", out.ID, -1)
				output = strings.Replace(output, "<aver>", out.AgentVersion, -1)
				output = strings.Replace(output, "<pver>", out.ProtocolVersion, -1)
				output = strings.Replace(output, "<pubkey>", out.PublicKey, -1)
				output = strings.Replace(output, "<addrs>", strings.Join(out.Addresses, "\n"), -1)
				output = strings.Replace(output, "\\n", "\n", -1)
				output = strings.Replace(output, "\\t", "\t", -1)
				fmt.Fprint(w, output)
			} else {
				marshaled, err := json.MarshalIndent(out, "", "\t")
				if err != nil {
					return err
				}
				marshaled = append(marshaled, byte('\n'))
				fmt.Fprintln(w, string(marshaled))
			}
			return nil
		}),
	},
	Type: IdOutput{},
}

func printPeer(ps pstore.Peerstore, p peer.ID) (interface{}, error) {
	if p == "" {
		return nil, errors.New("attempted to print nil peer")
	}

	info := new(IdOutput)
	info.ID = p.Pretty()

	if pk := ps.PubKey(p); pk != nil {
		pkb, err := ic.MarshalPublicKey(pk)
		if err != nil {
			return nil, err
		}
		info.PublicKey = base64.StdEncoding.EncodeToString(pkb)
	}

	for _, a := range ps.Addrs(p) {
		info.Addresses = append(info.Addresses, a.String())
	}

	if v, err := ps.Get(p, "ProtocolVersion"); err == nil {
		if vs, ok := v.(string); ok {
			info.ProtocolVersion = vs
		}
	}
	if v, err := ps.Get(p, "AgentVersion"); err == nil {
		if vs, ok := v.(string); ok {
			info.AgentVersion = vs
		}
	}

	return info, nil
}

//印刷本身是特殊情况下，因为我们得到的价值观不同。
func printSelf(node *core.IpfsNode) (interface{}, error) {
	info := new(IdOutput)
	info.ID = node.Identity.Pretty()

	pk := node.PrivateKey.GetPublic()
	pkb, err := ic.MarshalPublicKey(pk)
	if err != nil {
		return nil, err
	}
	info.PublicKey = base64.StdEncoding.EncodeToString(pkb)

	if node.PeerHost != nil {
		for _, a := range node.PeerHost.Addrs() {
			s := a.String() + "/ipfs/" + info.ID
			info.Addresses = append(info.Addresses, s)
		}
	}
	info.ProtocolVersion = identify.LibP2PVersion
	info.AgentVersion = identify.ClientVersion
	return info, nil
}
