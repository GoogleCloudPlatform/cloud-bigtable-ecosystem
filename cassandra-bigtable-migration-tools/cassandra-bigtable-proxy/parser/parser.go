package parser

import (
	"fmt"
	"strings"
	"sync"

	cql "github.com/GoogleCloudPlatform/cloud-bigtable-ecosystem/cassandra-bigtable-migration-tools/cassandra-bigtable-proxy/third_party/cqlparser"
	"github.com/antlr4-go/antlr/v4"
)

type syntaxErrorListener struct {
	*antlr.DefaultErrorListener
	errs []string
}

func (l *syntaxErrorListener) ValidateNoErrors() error {
	if len(l.errs) > 0 {
		return fmt.Errorf("parsing error: %s", strings.Join(l.errs, ", "))
	}
	return nil
}

// SyntaxError is called when the parser encounters a syntax error.
func (l *syntaxErrorListener) SyntaxError(
	recognizer antlr.Recognizer,
	offendingSymbol interface{},
	line, column int,
	msg string,
	e antlr.RecognitionException,
) {
	// Format a clear error message with location
	l.errs = append(l.errs, msg)
	recognizer.SetError(e)
}

type ProxyCqlParser struct {
	p             *cql.CqlParser
	lexer         *cql.CqlLexer
	stream        *antlr.CommonTokenStream
	errorListener *syntaxErrorListener
}

// parserPool holds reusable parser instances to drastically reduce GC pressure and CPU overhead.
var parserPool = sync.Pool{
	New: func() interface{} {
		// Pre-allocate slice capacity to prevent allocations during error appending
		errorListener := &syntaxErrorListener{
			DefaultErrorListener: antlr.NewDefaultErrorListener(),
			errs:                 make([]string, 0, 4),
		}

		// Initialize with empty input to setup the structure once
		input := antlr.NewInputStream("")
		lexer := cql.NewCqlLexer(input)
		lexer.RemoveErrorListeners()
		lexer.AddErrorListener(errorListener)

		stream := antlr.NewCommonTokenStream(lexer, antlr.TokenDefaultChannel)
		parser := cql.NewCqlParser(stream)
		parser.RemoveErrorListeners()
		parser.AddErrorListener(errorListener)
		// By default, ANTLR tries to magically recover from bad queries but this is slow. Disable it.
		parser.SetErrorHandler(antlr.NewBailErrorStrategy())
		// SLL ignores full context and runs exponentially faster, but occasionally fails on complex ambiguous syntax.
		parser.GetInterpreter().SetPredictionMode(antlr.PredictionModeSLL)
		return &ProxyCqlParser{
			p:             parser,
			lexer:         lexer,
			stream:        stream,
			errorListener: errorListener,
		}
	},
}

// GetParser pulls a parser from the pool and resets its state for the new query.
func GetParser(query string) *ProxyCqlParser {
	proxy := parserPool.Get().(*ProxyCqlParser)

	// Clear the errors without reallocating the underlying array
	proxy.errorListener.errs = proxy.errorListener.errs[:0]

	// Create a new input stream for the query and update the existing objects
	input := antlr.NewInputStream(query)
	proxy.lexer.SetInputStream(input)
	proxy.stream.SetTokenSource(proxy.lexer)
	proxy.p.SetTokenStream(proxy.stream)

	return proxy
}

// Release returns the parser back to the pool. MUST be called after parsing.
func (p *ProxyCqlParser) Release() {
	parserPool.Put(p)
}

// --- Wrapper Methods ---

func (p *ProxyCqlParser) ValidateNoErrors() error {
	return p.errorListener.ValidateNoErrors()
}

func (p *ProxyCqlParser) AlterTable() (cql.IAlterTableContext, error) {
	result := p.p.AlterTable()
	if err := p.ValidateNoErrors(); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *ProxyCqlParser) Delete_() (cql.IDelete_Context, error) {
	result := p.p.Delete_()
	if err := p.ValidateNoErrors(); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *ProxyCqlParser) Use_() (cql.IUse_Context, error) {
	result := p.p.Use_()
	if err := p.ValidateNoErrors(); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *ProxyCqlParser) Select_() (cql.ISelect_Context, error) {
	result := p.p.Select_()
	if err := p.ValidateNoErrors(); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *ProxyCqlParser) Truncate() (cql.ITruncateContext, error) {
	result := p.p.Truncate()
	if err := p.ValidateNoErrors(); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *ProxyCqlParser) DescribeStatement() (cql.IDescribeStatementContext, error) {
	result := p.p.DescribeStatement()
	if err := p.ValidateNoErrors(); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *ProxyCqlParser) CreateTable() (cql.ICreateTableContext, error) {
	result := p.p.CreateTable()
	if err := p.ValidateNoErrors(); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *ProxyCqlParser) DropTable() (cql.IDropTableContext, error) {
	result := p.p.DropTable()
	if err := p.ValidateNoErrors(); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *ProxyCqlParser) Update() (cql.IUpdateContext, error) {
	result := p.p.Update()
	if err := p.ValidateNoErrors(); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *ProxyCqlParser) Insert() (cql.IInsertContext, error) {
	result := p.p.Insert()
	if err := p.ValidateNoErrors(); err != nil {
		return nil, err
	}
	return result, nil
}

func (p *ProxyCqlParser) GetFirstToken() antlr.Token {
	return p.p.GetTokenStream().LT(1)
}
