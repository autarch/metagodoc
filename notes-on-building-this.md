## Thought 1

I will do this all from scratch. It's going to be the most beautiful code ever
written!

## Thought 2

This is a ridiculous amount of work. Why not use the packages in the godoc.org
source to do the heavy lifting?

## Problem

Godoc does not clone anything locally (at least for GitHub). Instead it uses
the GitHub API to recursively look through a repo's entire directory tree.

This burns through the GitHub API limit of 5,000 queries per hour remarkably
quickly. In my testing I would query GitHub for repos where "language=go" and
try to index each one in the returned list. This would use up all my queries
before I indexed the 30 repos returned by the results.

This obviously depends on repo size, but one of the repos in the initial
search results is https://github.com/aws/aws-sdk-go, which is ridiculously
large. But there are other large repos out there too.

GitHub reports that it  has 273,419 repos as of 2/4/2018. If  I can only index
10 repos  per hour,  then this  will take 27,341  hours, or  1,139 days,  or a
little over 3 years. Now maybe 10  repos per hour is too pessimistic, but even
at 100 repos per hour it'd take 114 days, which is more than 3 months!

That just won't work. I need a faster indexing method.

## Thought 3

I can clone the repos locally and use godoc APIs to index _that_.

## Problem

While godoc _can_ index a locally cloned repo, this is only intended for
development mode, and it produces only a fraction of the info I need.

## Thought 4

I will do this all from scratch. It's going to be the most beautiful code ever
written!

Except I'm going to liberally and shamelessly copy code from the godoc source
and use that as much as possible.

## Problem

The godoc code's abstractions are a bit confusing. It has a `gosrc.Directory`
struct which combines information about a repository and its top-level
directory. It also has a `doc.Package` struct which has much of the same info,
but is only for things which contain go code.

## Thought 5

Instead of copying the godoc code I can cannibilize it, at least.

## Problem

Git is a pain.

My first thought was to clone bare repos and use a package that can work with
repos directly to deal with them. This was intended to save disk space. A bare
clone be about 70-80% of the size of a regular clone from my testing. This
adds up over thousands of repos!

However, I realized that the go/* packages all require files on disk. That
meant I'd have to check out each branch and tag I wanted to index somehow. At
first, I started cloning my bare repos with `--depth=1` but that ends up using
more space than just doing a regular clone in the first place (no
surprise). So I switched to using regular clones and working with them in
place to check out branches and tags.

I tried a number of different tools for interfacing with git repos. I tried
for quite some time to get https://gopkg.in/src-d/go-git.v4 working, and got
most of the way there, but then I'd get very mysterious messages like "object
not found" for operations like trying to check out a tag. Trying to debug this
is very challenging unless you understand Git at a very deep level, which I
don't.

However, I do know how to do all the operations I want using the `git` CLI
tool. So why not use that? I found https://github.com/go-gitea/git, which is a
nice wrapper around calling `git `directly. It lets you fall back to calling
arbitrary commands for anything it doesn't wrap. Perfect!

## Aside

I did a bunch of work on my laptop, then pushed to master. Then I came back a
couple weeks later on my desktop, did a bunch of work and tried to pull. The
new work had a huge overlap with the old work, but I'd solved the same
problems differently. Apparently my design process is not a reproducible
build.
