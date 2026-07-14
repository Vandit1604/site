---
title: "I Built a File System Where Nobody Can Lie to You"
date: "2025-10-02"
tags: ["tech", "golang", "web3"]
---

Here is a question that sounds dumb but isn't: when someone sends you a file over the network, how do you *know* it's the file you asked for? Not "the server pinky-promised." Actually know.

Normally you don't. You ask `server.com` for `report.pdf`, it hands you some bytes, and you just trust it. If the server swapped the bytes, corrupted them in transit, or got popped last Tuesday, you have no way to tell. You named the file, and the name is a label anybody can slap on anything.

I got a little obsessed with fixing that and built [Phile Storage](https://github.com/Vandit1604/phile-storage), a peer-to-peer file system in Go where **a peer physically cannot hand you tampered data**. In this post I'll walk through the whole thing: the one idea that makes it work, the actual Go code that enforces it, how peers find each other with no server in the middle, and where the design bites back. By the end you'll be able to build a tiny version yourself.

Here's the map:

- What content addressing is, and why it changes the trust model
- The ~40 lines of Go that turn bytes into an address
- How a fetch gets verified so lying becomes mathematically impossible
- How peers discover each other over libp2p with zero infrastructure
- The tradeoffs, because there are always tradeoffs

Let's go.

## The core idea: stop naming files, start fingerprinting them

The whole system rests on one move called **content addressing**, and it's a genuine vibe shift. Instead of addressing a file by a name you chose, you address it by the **hash of its actual bytes**. The address *is* the fingerprint.

<figure>
  <img src="/static/images/blog/05-content-addressing.svg" alt="Same bytes hashed with SHA-256 always give the same CID; changing one byte gives a completely different CID.">
  <figcaption>The bytes decide the address. Change one byte and the address is unrecognizable, so the name can never drift from the content.</figcaption>
</figure>

In Phile that fingerprint is a **CID** (content identifier). The rules for making one are fixed up front: CID version 1, a codec that says "these are just raw bytes," and SHA-256 as the hash.

```go
// rawCodec marks a CID as pointing at opaque bytes (not a DAG node).
const rawCodec = 0x55

var prefix = cid.Prefix{
    Version:  1,
    Codec:    rawCodec,
    MhType:   mh.SHA2_256,
    MhLength: -1,
}

// Compute returns the CID for a block of bytes.
func Compute(data []byte) (cid.Cid, error) {
    return prefix.Sum(data)
}
```

<a class="src-link" href="https://github.com/Vandit1604/phile-storage/blob/main/backend/internal/content/cid.go" target="_blank" rel="noopener noreferrer">↗ backend/internal/content/cid.go</a>

Feed in bytes, get back a CID. Same bytes always produce the same CID. Change one pixel, one comma, one byte, and you get a completely different CID. The name and the content are now welded together. You cannot pry them apart.

This one decision buys you three things basically for free:

- **Dedup.** Two people upload the identical file? Same bytes, same CID, stored once. You didn't write any "check for duplicates" logic. The math just does it.
- **Immutability.** A CID points at *exactly one* possible set of bytes, forever. There is no "update the file at this address." A different file is a different address, full stop.
- **Verification.** And this is the good one. Keep reading.

<aside class="callout" data-label="Note">
A CID isn't just the raw hash. It's the hash plus a little self-describing header (version, codec, hash type), all in one string. That's why a peer receiving a CID knows exactly how to re-derive it without you telling it anything out of band. Self-describing formats are underrated.
</aside>

## The part where lying becomes impossible

Here's the magic trick. If the address is the hash of the content, then anyone who receives content can just re-hash it and check it matches the address they asked for. Two tiny functions do this:

```go
// Verify reports whether data hashes to want.
func Verify(data []byte, want cid.Cid) bool {
    got, err := Compute(data)
    if err != nil {
        return false
    }
    return got.Equals(want)
}
```

<a class="src-link" href="https://github.com/Vandit1604/phile-storage/blob/main/backend/internal/content/cid.go" target="_blank" rel="noopener noreferrer">↗ backend/internal/content/cid.go</a>

Now watch where that gets used. When a Phile node fetches a block from some random peer across the network, the very last thing it does before returning the bytes is verify them:

```go
// Fetch pulls a block from a provider over a stream and returns it only if the
// bytes hash to c. Wrong or tampered content is rejected, never returned.
func (n *Node) Fetch(ctx context.Context, c cid.Cid, from peer.AddrInfo) ([]byte, error) {
    // ... connect, open the stream, send the CID we want ...

    data, err := io.ReadAll(s)
    if err != nil {
        return nil, fmt.Errorf("read block: %w", err)
    }
    if !content.Verify(data, c) {
        return nil, fmt.Errorf("integrity check failed for %s", c)
    }
    return data, nil
}
```

<a class="src-link" href="https://github.com/Vandit1604/phile-storage/blob/main/backend/internal/p2p/fetch.go" target="_blank" rel="noopener noreferrer">↗ backend/internal/p2p/fetch.go</a>

Think about what that `if` statement actually means for an attacker.

<figure>
  <img src="/static/images/blog/06-fetch-verify.svg" alt="A node asks the DHT for a CID, opens a stream to a provider, receives bytes, re-hashes them, and keeps the block only if the hash matches the CID.">
  <figcaption>Every cross-peer fetch ends in a re-hash. Match, keep it. Mismatch, it hits the floor.</figcaption>
</figure>

A malicious peer wants to feed you garbage. But you asked for CID `bafkreib...a1c3`. For your check to pass, the garbage they send would have to hash to `bafkreib...a1c3`. That's the same as asking them to find a SHA-256 collision on demand, which, lol, no. So they can't. **Corrupted or swapped content just gets rejected on arrival, automatically.** Trust isn't a feeling here. It's arithmetic.

<aside class="callout callout--tip" data-label="Why it matters">
This flips the security model. In the normal web you trust <em>the source</em>: is this really my bank's server? With content addressing you trust <em>the data</em>: do these bytes hash to what I asked for? The source can be a total stranger and it doesn't matter. That's what makes peer-to-peer distribution actually safe instead of terrifying.
</aside>

## Okay but how does it find the file at all?

Cool, the bytes are verifiable. But there's no central server. So when I ask for a CID, who do I even talk to? This is where [libp2p](https://libp2p.io/) does the heavy lifting.

Every node has a **PeerID**, a cryptographic identity derived from a keypair, saved to `identity.key` so it survives restarts. Your node is the same "person" every time it boots. Nodes find each other two ways:

- **mDNS** for peers on your local network. The "shout on the LAN and see who answers" approach.
- A **Kademlia DHT** for the wider network. A DHT is a distributed phone book with no owner. When my node stores a block, it announces to the DHT "hey, I have this CID," and anyone can later ask the DHT who has it.

The announce and lookup are two small methods:

```go
// Provide announces to the DHT that this node holds c.
func (n *Node) Provide(ctx context.Context, c cid.Cid) error {
    if err := n.dht.Provide(ctx, c, true); err != nil {
        return fmt.Errorf("provide %s: %w", c, err)
    }
    return nil
}

// FindProviders asks the network who holds c, returning up to max providers.
func (n *Node) FindProviders(ctx context.Context, c cid.Cid, max int) []peer.AddrInfo {
    out := make([]peer.AddrInfo, 0, max)
    for pi := range n.dht.FindProvidersAsync(ctx, c, max) {
        if pi.ID == n.host.ID() { // skip myself
            continue
        }
        out = append(out, pi)
    }
    return out
}
```

<a class="src-link" href="https://github.com/Vandit1604/phile-storage/blob/main/backend/internal/p2p/fetch.go" target="_blank" rel="noopener noreferrer">↗ backend/internal/p2p/fetch.go</a>

To download, I ask the DHT `FindProviders` for a CID, it points me at peers holding it, and I open a direct stream to one of them over a little custom protocol I named `/phile/fetch/1.0.0`. They send bytes, I re-hash (see above), and either it checks out or it hits the floor.

## Run it yourself

The fun part: the whole thing runs with **zero external infrastructure by default.** No database to babysit, no S3 bill, no central index.

```bash
git clone https://github.com/Vandit1604/phile-storage
cd phile-storage/backend
make build
./bin/phile-storage -peers=3   # spins up 3 peers on localhost
```

Upload a file to peer 1, then fetch it by CID from peer 3. It was never copied directly between them by you. Peer 3 asked the DHT, found peer 1, streamed the block, and verified it. Blocks live on disk under `blocks/<cid>` and get re-announced to the DHT on startup, so a node can reboot and rejoin the network right where it left off.

There's an *optional* centralized mode (etcd + Redis) behind a single env var, for when you actually want one global searchable index. But the version that made me build this needs nothing but the peers themselves.

## Where the design bites back

I'm not going to pretend content addressing is free lunch. The honest tradeoffs:

- **Immutability cuts both ways.** "The file at this address can never change" is a feature until you want to change the file. Now you need a *mutable pointer* that says "the latest version is this CID," which is a whole separate naming problem (IPFS solves it with IPNS, and it's the gnarly part).
- **Content addressing doesn't give you discovery.** A CID tells you nothing about what the file *is*. You still need some human-readable name to CID map somewhere, and in decentralized mode each node only knows the names of files it uploaded.
- **The DHT is eventually-consistent and can be slow.** `FindProviders` might take a beat, or briefly return nobody if an announce hasn't propagated. Great for resilience, occasionally annoying for latency.

<aside class="callout callout--warn" data-label="Gotcha">
Verifying on arrival protects integrity, not availability. Nobody can hand you <em>wrong</em> bytes, but if every peer holding a CID goes offline, that content is simply gone. Content addressing guarantees "you get the right file or nothing," never "you always get the file."
</aside>

## TL;DR

- Address files by the **hash of their bytes** (a CID), not by a name. Name and content become inseparable.
- You get **dedup and immutability for free**, and every download is **re-hashed and verified**, so tampered bytes are auto-rejected. Lying is mathematically off the table.
- **libp2p** handles identity (PeerID), discovery (mDNS + Kademlia DHT), and a direct fetch stream. No central server.
- The catch: immutability makes updates hard, and integrity is not the same as availability.

Content addressing flips the trust model. You stop trusting *who* sent the data and start trusting the *data itself*. Once that clicks, plain old "here's a file at this URL, trust me bro" starts to feel kind of medieval.

## Go deeper

- The Phile Storage repo: [github.com/Vandit1604/phile-storage](https://github.com/Vandit1604/phile-storage) (the CID logic is [~40 lines](https://github.com/Vandit1604/phile-storage/blob/main/backend/internal/content/cid.go))
- [libp2p docs](https://docs.libp2p.io/) and the [Kademlia DHT spec](https://docs.libp2p.io/concepts/discovery-routing/kaddht/)
- [How IPFS works](https://docs.ipfs.tech/concepts/how-ipfs-works/), which uses this exact model at planet scale
- The [CID spec](https://github.com/multiformats/cid) if you want to see what's really inside that string

---

*Fun fact: this is the same core idea behind IPFS and, honestly, git. Every git commit hash is a content address. That's why you can't quietly rewrite history without every downstream hash changing. You've been using content addressing this whole time. Go poke at [the code](https://github.com/Vandit1604/phile-storage), the CID part really is like 50 lines.*
