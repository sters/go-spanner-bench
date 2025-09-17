-- BenchBase: Base table without search index
CREATE TABLE BenchBase (
    UUID STRING(36) NOT NULL,
    Body STRING(MAX) NOT NULL
) PRIMARY KEY (UUID);

-- BenchFulltext: Table with FULLTEXT search index
CREATE TABLE BenchFulltext (
    UUID STRING(36) NOT NULL,
    Body STRING(MAX) NOT NULL,
    BodyTokens TOKENLIST AS (TOKENIZE_FULLTEXT(Body)) HIDDEN
) PRIMARY KEY (UUID);

CREATE SEARCH INDEX BenchFulltext_BodyTokens ON BenchFulltext(BodyTokens);

-- BenchSubstring: Table with SUBSTRING search index
CREATE TABLE BenchSubstring (
    UUID STRING(36) NOT NULL,
    Body STRING(MAX) NOT NULL,
    BodyTokens TOKENLIST AS (TOKENIZE_SUBSTRING(Body)) HIDDEN
) PRIMARY KEY (UUID);

CREATE SEARCH INDEX BenchSubstring_BodyTokens ON BenchSubstring(BodyTokens);

-- BenchNgrams: Table with NGRAMS search index (2-3 grams)
CREATE TABLE BenchNgrams (
    UUID STRING(36) NOT NULL,
    Body STRING(MAX) NOT NULL,
    BodyTokens TOKENLIST AS (TOKENIZE_NGRAMS(Body, ngram_size_min=>2, ngram_size_max=>3)) HIDDEN
) PRIMARY KEY (UUID);

CREATE SEARCH INDEX BenchNgrams_BodyTokens ON BenchNgrams(BodyTokens);