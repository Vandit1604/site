---
title: "I Built a File System Where Nobody Can Lie to You"
date: "2025-10-02"
tags: ["tech", "golang", "web3"]
---

Here is a question that sounds dumb but isn't: when someone sends you a file over the network, how do you *know* it's the file you asked for? Not "the server pinky-promised." Actually know.

Normally you don't. You ask `server.com` for `report.pdf`, it hands you some bytes, and you just... trust it. If the server swapped the bytes, corrupted them, or got hacked, you have no way to tell. You named the file, and the name is a label anybody can slap on anything.

I got a little obsessed with fixing that and ended up building [Phile Storage](https://github.com/Vandit1604/phile-storage), a peer-to-peer file system in Go where **a peer physically cannot hand you tampered data**. Let me explain the one idea that makes it work, because it's genuinely elegant.

## Stop naming files. Fingerprint them.

The core move is called **content addressing**, and it's a whole vibe shift. Instead of addressing a file by a name you chose, you address it by the **hash of its actual bytes**. The address *is* the fingerprint.

In Phile that fingerprint is a CID (content identifier). Making one is boring in the best way:

```go
var prefix = cid.Prefix{
    Version:  1,       // CIDv1
    Codec:    rawCodec, // 0x55, "these are just raw bytes"
    MhType:   mh.SHA2_256,
    MhLength: -1,
}

func Compute(data []byte) (cid.Cid, error) {
    return prefix.Sum(data)
}
```

Feed in bytes, get back a CID. Same bytes always give the same CID. Change one pixel, one comma, one byte, and you get a completely different CID. The name and the content are now welded together. You cannot pry them apart.

This one decision buys you three things for free:

- **Dedup.** Two people upload the identical file? Same bytes, same CID, stored once. You didn't write any "check for duplicates" logic. The math just does it.
- **Immutability.** A CID points at *exactly one* possible set of bytes, forever. There's no "update the file at this address." A different file is a different address.
- **Verification.** And this is the good one, keep reading.

## The part where lying becomes impossible

Here's the magic trick. If the address is the hash of the content, then anyone who receives content can just... re-hash it and check it matches the address they asked for.

So when a Phile peer fetches a block from some random other peer across the network, the very last thing it does before saving is this:

```go
if !content.Verify(data, c) {
    // the bytes don't hash to the CID we asked for. drop them.
    return errMismatch
}
```

Think about what that means. A malicious peer wants to feed you garbage. But you asked for CID `bafk...abc`. For your check to pass, the garbage they send would have to hash to `bafk...abc`. That's the same as asking them to find a SHA-256 collision on demand, which, lol, no. So they can't. **Corrupted or swapped content just gets rejected on arrival, automatically.** Trust isn't a feeling here, it's arithmetic.

## Okay but how does it find the file at all

Cool, the bytes are verifiable. But there's no central server. So when I ask for a CID, who do I even talk to? This is where [libp2p](https://libp2p.io/) does the heavy lifting.

Every node has a **PeerID**, a cryptographic identity derived from a keypair, saved to `identity.key` so it survives restarts (your node is the same "person" every time it boots). Nodes find each other two ways:

- **mDNS** for peers on your local network, the "shout on the LAN and see who answers" approach.
- A **Kademlia DHT** for the wider network. A DHT is a distributed phone book with no owner. When my node stores a block, it tells the DHT "hey, I have CID `bafk...abc`":

```go
n.dht.Provide(ctx, c, true)          // I have this block
providers := n.FindProviders(ctx, c) // who else has this one?
```

To download, I ask the DHT `FindProviders` for a CID, it points me at peers holding it, and I open a direct stream to one of them over a little custom protocol I named `/phile/fetch/1.0.0`. They send bytes, I re-hash (see above), and either it checks out or it hits the floor.

## The bit I'm smug about

The whole system runs with **zero external infrastructure by default.** No database to babysit, no S3 bill, no central index. Peers discover each other, announce what they hold, and move content directly. Blocks live on disk under `blocks/<cid>` and get re-announced to the DHT on startup, so a node rejoins the network and picks right back up.

There's an *optional* centralized mode (etcd + Redis) behind a single env var, for when you actually want one global searchable index. But the fun version, the one that made me build this, needs nothing but the peers themselves.

## TL;DR

- Address files by the **hash of their bytes** (a CID), not by a name. Name and content become inseparable.
- Free dedup, free immutability, and every download is **re-hashed and verified**, so tampered bytes get auto-rejected. Lying is mathematically off the table.
- **libp2p** handles identity (PeerID), discovery (mDNS + Kademlia DHT), and a direct fetch stream. No central server.

Content addressing flips the trust model. You stop trusting *who* sent the data and start trusting the *data itself*. Once that clicks, plain old "here's a file at this URL, trust me bro" starts to feel kind of medieval.

---

*Fun fact: this is the same core idea behind IPFS and, honestly, git. Every git commit hash is a content address, that's why you can't quietly rewrite history without every downstream hash changing. You've been using content addressing this whole time. Go poke at [the code](https://github.com/Vandit1604/phile-storage), the CID part is like 50 lines.*
