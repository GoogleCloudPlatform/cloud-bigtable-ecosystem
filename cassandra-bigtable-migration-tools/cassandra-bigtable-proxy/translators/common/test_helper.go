package common

import (
	// Import your generated parser package
	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/antlr4-go/antlr/v4"
)

// ErrorListener captures syntax errors so we can assert on them
type TestErrorListener struct {
	*antlr.DefaultErrorListener
	Errors []string
}

func (l *TestErrorListener) SyntaxError(recognizer antlr.Recognizer, offendingSymbol interface{}, line, column int, msg string, e antlr.RecognitionException) {
	l.Errors = append(l.Errors, msg)
}

// Helper to setupParser parser for a string
func setupParser(input string) (*cql.CqlParser, *TestErrorListener) {
	is := antlr.NewInputStream(input)

	// Setup Lexer
	lexer := cql.NewCqlLexer(is)
	stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)

	// Setup Parser
	p := cql.NewCqlParser(stream)

	// Attach custom error listener
	l := &TestErrorListener{}
	p.RemoveErrorListeners() // remove console logger
	p.AddErrorListener(l)

	return p, l
}
