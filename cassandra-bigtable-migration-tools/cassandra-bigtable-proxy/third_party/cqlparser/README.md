# Regenerating CQLParser

If the CQL parser doesn't support the CQL keywords that you need, you can update
one of the .g4 files and then regenerate the golang parsers with:

1. Download the antlr 4
   jar: `curl -O https://www.antlr.org/download/antlr-4.13.2-complete.jar`
2. `cd` into `third_party/cqlparser/`
3.
Run `java -jar ~/Downloads/antlr-4.13.2-complete.jar -Dlanguage=Go -no-visitor -package parser CqlLexer.g4 CqlParser.g4`
