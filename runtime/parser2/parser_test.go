/*
 * Cadence - The resource-oriented smart contract programming language
 *
 * Copyright 2019-2020 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package parser2

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/onflow/cadence/runtime/ast"
	"github.com/onflow/cadence/runtime/parser2/lexer"
	"github.com/onflow/cadence/runtime/tests/utils"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestParseInvalid(t *testing.T) {

	t.Parallel()

	_, err := ParseProgram("X")
	require.EqualError(t, err, "Parsing failed:\nerror: unexpected token: identifier\n --> :1:0\n  |\n1 | X\n  | ^\n")
}

func TestParseBuffering(t *testing.T) {

	t.Parallel()

	t.Run("buffer and accept, valid", func(t *testing.T) {

		t.Parallel()

		_, errs := Parse("a b c d", func(p *parser) interface{} {
			p.mustOneString(lexer.TokenIdentifier, "a")
			p.mustOne(lexer.TokenSpace)

			p.startBuffering()

			p.mustOneString(lexer.TokenIdentifier, "b")
			p.mustOne(lexer.TokenSpace)
			p.mustOneString(lexer.TokenIdentifier, "c")

			p.acceptBuffered()

			p.mustOne(lexer.TokenSpace)
			p.mustOneString(lexer.TokenIdentifier, "d")

			return nil
		})

		assert.Empty(t, errs)
	})

	t.Run("buffer and accept, invalid", func(t *testing.T) {

		t.Parallel()

		_, errs := Parse("a b x d", func(p *parser) interface{} {
			p.mustOneString(lexer.TokenIdentifier, "a")
			p.mustOne(lexer.TokenSpace)

			p.startBuffering()

			p.mustOneString(lexer.TokenIdentifier, "b")
			p.mustOne(lexer.TokenSpace)
			p.mustOneString(lexer.TokenIdentifier, "c")

			p.acceptBuffered()

			p.mustOne(lexer.TokenSpace)
			p.mustOneString(lexer.TokenIdentifier, "d")

			return nil
		})

		utils.AssertEqualWithDiff(t,
			[]error{
				&SyntaxError{
					Message: "expected token identifier with string value c",
					Pos:     ast.Position{Offset: 4, Line: 1, Column: 4},
				},
			},
			errs,
		)
	})

	t.Run("buffer and replay, valid", func(t *testing.T) {

		t.Parallel()

		_, errs := Parse("a b c d", func(p *parser) interface{} {
			p.mustOneString(lexer.TokenIdentifier, "a")
			p.mustOne(lexer.TokenSpace)

			p.startBuffering()

			p.mustOneString(lexer.TokenIdentifier, "b")
			p.mustOne(lexer.TokenSpace)
			p.mustOneString(lexer.TokenIdentifier, "c")

			p.replayBuffered()

			p.mustOneString(lexer.TokenIdentifier, "b")
			p.mustOne(lexer.TokenSpace)
			p.mustOneString(lexer.TokenIdentifier, "c")
			p.mustOne(lexer.TokenSpace)
			p.mustOneString(lexer.TokenIdentifier, "d")

			return nil
		})

		assert.Empty(t, errs)
	})

	t.Run("buffer and replay, invalid first", func(t *testing.T) {

		t.Parallel()

		_, errs := Parse("a b c d", func(p *parser) interface{} {
			p.mustOneString(lexer.TokenIdentifier, "a")
			p.mustOne(lexer.TokenSpace)

			p.startBuffering()

			firstSucceeded := false
			firstFailed := false

			(func() {
				defer func() {
					if r := recover(); r != nil {
						firstFailed = true
					}
				}()

				p.mustOneString(lexer.TokenIdentifier, "x")
				p.mustOne(lexer.TokenSpace)
				p.mustOneString(lexer.TokenIdentifier, "c")

				firstSucceeded = true
			})()

			assert.True(t, firstFailed)
			assert.False(t, firstSucceeded)

			p.replayBuffered()

			p.mustOneString(lexer.TokenIdentifier, "b")
			p.mustOne(lexer.TokenSpace)
			p.mustOneString(lexer.TokenIdentifier, "c")
			p.mustOne(lexer.TokenSpace)
			p.mustOneString(lexer.TokenIdentifier, "d")

			return nil
		})

		assert.Empty(t, errs)
	})

	t.Run("buffer and replay, invalid first and invalid second", func(t *testing.T) {

		t.Parallel()

		_, errs := Parse("a b c x", func(p *parser) interface{} {
			p.mustOneString(lexer.TokenIdentifier, "a")
			p.mustOne(lexer.TokenSpace)

			p.startBuffering()

			firstSucceeded := false
			firstFailed := false

			(func() {
				defer func() {
					if r := recover(); r != nil {
						firstFailed = true
					}
				}()

				p.mustOneString(lexer.TokenIdentifier, "x")
				p.mustOne(lexer.TokenSpace)
				p.mustOneString(lexer.TokenIdentifier, "c")

				firstSucceeded = true
			})()

			assert.True(t, firstFailed)
			assert.False(t, firstSucceeded)

			p.replayBuffered()

			p.mustOneString(lexer.TokenIdentifier, "b")
			p.mustOne(lexer.TokenSpace)
			p.mustOneString(lexer.TokenIdentifier, "c")
			p.mustOne(lexer.TokenSpace)
			p.mustOneString(lexer.TokenIdentifier, "d")

			return nil
		})

		utils.AssertEqualWithDiff(t,
			[]error{
				&SyntaxError{
					Message: "expected token identifier with string value d",
					Pos:     ast.Position{Offset: 6, Line: 1, Column: 6},
				},
			},
			errs,
		)
	})

	t.Run("nested buffering, invalid", func(t *testing.T) {

		t.Parallel()

		src := `
        transaction { }
        pub fun main() {
            assert(isneg(x:-1.0))
            assert(!isneg(x:-0.0/0.0))
        }
        pub fun isneg(x:SignedFixedPoint):Bool { /* I kinda forget what this is all about */
            return x                             /* but we probably need to figure it out */
                   <                             /* ************/((TODO?{/*))************ *//
                  -x                             /* maybe it says NaNs are not negative?  */
        }`

		_, err := ParseProgram(src)
		utils.AssertEqualWithDiff(t,
			[]error{
				&SyntaxError{
					Message: "expected token identifier",
					Pos:     ast.Position{Offset: 371, Line: 10, Column: 12},
				},
			},
			err.(Error).Errors,
		)
	})

	t.Run("nested buffering, invalid; apparent invocation elision", func(t *testing.T) {

		t.Parallel()

		src := `
        transaction { }
        pub fun main() {
            fun abs(_:Int):Int { return _ > 0 ? _ : -_ }
            let sanity = 0 <          /*****/((TODO?{/*****//
                             abs(-1)
            assert(sanity)
        }`

		_, err := ParseProgram(src)
		utils.AssertEqualWithDiff(t,
			[]error{
				&SyntaxError{
					Message: "expected token '/'",
					Pos:     ast.Position{Offset: 159, Line: 6, Column: 20},
				},
			},
			err.(Error).Errors,
		)
	})

	t.Run("nested buffering, valid; accept,accept,replay", func(t *testing.T) {

		t.Parallel()

		src := `
            pub struct interface Y {}
            pub struct X : Y {}
            pub fun main():String {
                fun f(a:Bool, _:String):String { return _; }
                let S = 1
                if false {
                    let Type_X_Y__qp_identifier =
                                    Type<X{Y}>().identifier; // parses fine
                    return f(a:S<S, Type_X_Y__qp_identifier)
                } else {
                    return f(a:S<S, Type<X{Y}>().identifier) // should also parse fine
                }
            }`

		_, err := ParseProgram(src)
		assert.NoError(t, err)
	})

	t.Run("nested buffering, valid; overlapped", func(t *testing.T) {

		t.Parallel()

		src := `
            transaction { }
            pub fun main():String {
                let A = 1
                let B = 2
                let C = 3
                let D = 4
                fun g(a:Bool, _:Bool):String { return _ ? "y" : "n" }
                return g(a:A<B, C<(D>>(5)))
            }`

		_, err := ParseProgram(src)
		assert.NoError(t, err)
	})

}

func TestParseEOF(t *testing.T) {

	t.Parallel()

	_, errs := Parse("a b", func(p *parser) interface{} {
		p.mustOneString(lexer.TokenIdentifier, "a")
		p.skipSpaceAndComments(true)
		p.mustOneString(lexer.TokenIdentifier, "b")

		p.next()

		assert.Equal(t,
			lexer.Token{
				Type: lexer.TokenEOF,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 3, Line: 1, Column: 3},
					EndPos:   ast.Position{Offset: 3, Line: 1, Column: 3},
				},
			},
			p.current,
		)

		p.next()

		assert.Equal(t,
			lexer.Token{
				Type: lexer.TokenEOF,
				Range: ast.Range{
					StartPos: ast.Position{Offset: 3, Line: 1, Column: 3},
					EndPos:   ast.Position{Offset: 3, Line: 1, Column: 3},
				},
			},
			p.current,
		)

		return nil
	})

	assert.Empty(t, errs)
}

func TestParseNames(t *testing.T) {

	t.Parallel()

	names := map[string]bool{
		// Valid: title-case
		//
		"PersonID": true,

		// Valid: with underscore
		//
		"token_name": true,

		// Valid: leading underscore and characters
		//
		"_balance": true,

		// Valid: leading underscore and numbers
		"_8264": true,

		// Valid: characters and number
		//
		"account2": true,

		// Invalid: leading number
		//
		"1something": false,

		// Invalid: invalid character #
		"_#1": false,

		// Invalid: various invalid characters
		//
		"!@#$%^&*": false,
	}

	for name, validExpected := range names {

		code := fmt.Sprintf(`let %s = 1`, name)

		actual, err := ParseProgram(code)

		if validExpected {
			assert.NotNil(t, actual)
			assert.NoError(t, err)

		} else {
			assert.Nil(t, actual)
			assert.IsType(t, Error{}, err)
		}
	}
}

func TestParseArgumentList(t *testing.T) {

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()

		_, errs := ParseArgumentList(`xyz`)
		utils.AssertEqualWithDiff(t,
			[]error{
				&SyntaxError{
					Message: "expected token '('",
					Pos:     ast.Position{Offset: 0, Line: 1, Column: 0},
				},
			},
			errs,
		)
	})

	t.Run("empty", func(t *testing.T) {
		t.Parallel()

		result, errs := ParseArgumentList(`()`)
		require.Empty(t, errs)

		var expected ast.Arguments

		utils.AssertEqualWithDiff(t,
			expected,
			result,
		)
	})

	t.Run("valid", func(t *testing.T) {
		t.Parallel()

		result, errs := ParseArgumentList(`(1, b: true)`)
		require.Empty(t, errs)

		utils.AssertEqualWithDiff(t,
			ast.Arguments{
				{
					Label:         "",
					LabelStartPos: nil,
					LabelEndPos:   nil,
					Expression: &ast.IntegerExpression{
						PositiveLiteral: "1",
						Value:           big.NewInt(1),
						Base:            10,
						Range: ast.Range{
							StartPos: ast.Position{
								Offset: 1,
								Line:   1,
								Column: 1,
							},
							EndPos: ast.Position{
								Offset: 1,
								Line:   1,
								Column: 1,
							},
						},
					},
					TrailingSeparatorPos: ast.Position{
						Offset: 2,
						Line:   1,
						Column: 2,
					},
				},
				{
					Label: "b",
					LabelStartPos: &ast.Position{
						Offset: 4,
						Line:   1,
						Column: 4,
					},
					LabelEndPos: &ast.Position{
						Offset: 4,
						Line:   1,
						Column: 4,
					},
					Expression: &ast.BoolExpression{
						Value: true,
						Range: ast.Range{
							StartPos: ast.Position{
								Offset: 7,
								Line:   1,
								Column: 7,
							},
							EndPos: ast.Position{
								Offset: 10,
								Line:   1,
								Column: 10,
							},
						},
					},
					TrailingSeparatorPos: ast.Position{
						Offset: 11,
						Line:   1,
						Column: 11,
					},
				},
			},
			result,
		)
	})

}
