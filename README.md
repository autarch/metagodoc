# MetaGodoc

This is an attempt to create an alternative to https://godoc.org with more
features and stuff.

The primary inspiration is Perl's https://metacpan.org site, which provides a
search engine along with a display of all sorts of useful info about the
packages it indexes.

Like MetaCPAN, this project uses Elasticsearch as the primary backend for
indexing information about packages. If it ever evolves to the point of having
logins and such we may also need a SQL database of somet sort (or maybe not).

## Random Thoughts on Future Features

Here's a brain dump of things I'd like to achieve with this project ...

### All Versions Indexed

For packages which are tagging releases, it'd be really nice to allow users to
browse the documentation of each release. We'll always want to index the
`HEAD` of the default branch as well, and that should be the default view for
most packages.

How this plays nicely with packages that use gopkg.in is very much TBD, but
obviously it should.

### Rich Metadata

One of the things I really like about MetaCPAN is the sidebar for a
package. See https://metacpan.org/release/Moose for example. The left hand
side links to its changelog, website, git repo, reported issues, package
review, and more.

While Golang packages don't have all those things (changelogs, sigh), we can
certainly link to the repo, offer information about issues, etc.

### Better Documentation Organization

The default organization of go Package documentation is not very
helpful. Presenting things in an arbitrary order is generally not the best way
for users to understand the package.

This is especially problematic for monster packages like
<https://godoc.org/gopkg.in/olivere/elastic.v6>. This is so huge that Chrome
has trouble scrolling it sanely.

And finding anything in it is really difficult. Now, in part, that's the fault
of the package author. Splitting this one package up into multiple packages
would greatly improve the presentation.

However, it would still be nice to allow package authors more control over the
presentation of their documentation.

My vague thought is that package authors could add a `metagodoc.json` file to
their repo that would provide instructions on how to present their
documentation. For example, they could specify something like "show the docs
for these symbols" first or even provide a more sophisticated grouping of
symbols to be split up across multiple pages.

This is all very handwavy at this point, but it's something that is really
needed.

### Better Search

I think using Elastic has the *potential* to make for much better search
results. We can consider many things:

* Location of words in the text (package name vs synopsis vs buried somewhere
  in there).
* Other content in the repo, such as `README` files and other docs.
* Import count - how many other packages use this one? Can we come up with a
  River of Go like [the River of
  CPAN](http://neilb.org/2015/04/20/river-of-cpan.html)?
* Fork vs not fork.
* How active is the repository?

And yes, I know godoc.org already factors some of those things in.

### Modern UI

At the very least, a wider central column.

### Random Other Stuff

It'd be nice to be able to see all the repos from a single author, like
[MetaCPAN's author view](https://metacpan.org/author/DROLSKY).

Insert Your Feature here.

