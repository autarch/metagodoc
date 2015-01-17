package indexer

import (
	"log"
	"net/url"

	elastigo "github.com/mattbaird/elastigo/lib"
)

type Indexer struct {
	conn *elastigo.Conn
}

type Package struct {
	Name       string `json:"name"`
	PrimaryUrl string `json:"primary_url"`
	Author     string `json:"author"`
}

func New() *Indexer {
	conn := elastigo.NewConn()
	return &Indexer{conn}
}

func (idx *Indexer) Index() {
	u, err := url.Parse("https://github.com/mattbaird/elastigo/lib")
	if err != nil {
		panic(err)
	}

	r, err := idx.conn.Index(
		"gopal",
		"package",
		"",
		nil,
		Package{
			Name:       "elastigo",
			PrimaryUrl: u.String(),
			Author:     "mattbaird",
		},
	)

	if err != nil {
		panic(err)
	}

	log.Print(r)
}
