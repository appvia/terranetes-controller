/*
 * Copyright (C) 2022  Appvia Ltd <info@appvia.io>
 *
 * This program is free software; you can redistribute it and/or
 * modify it under the terms of the GNU General Public License
 * as published by the Free Software Foundation; either version 2
 * of the License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package cmd

import (
	"fmt"
	"io"
)

// defaultFormatter is the default formatter for the cli
type defaultFormatter struct{}

// defaultJSONFormatter is the default json formatter for the cli 
type defaultJSONFormatter struct{} 

// NewTextFormatter returns a new text formatter 
func NewTextFormatter() Formatter { 
	return &defaultFormatter{} 
}

// NewJSONFormatter returns a new json formatter 
func NewJSONFormatter() Formatter { 
	return &defaultJSONFormatter{} 
}

// Printf prints a message to the output stream 
func (f *defaultJSONFormatter) Printf(out io.Writer, format string, a ...interface{}) {
	fmt.Fprintf(out, "{ \"message\": \"" + format + "\" }", a...)
}

// Println prints a message to the output stream 
func (f *defaultJSONFormatter) Println(out io.Writer, format string, a ...interface{}) { 
	fmt.Fprintf(out, "{ \"message\": \"" + format + "\" }", a...)
}

// Printf prints a message to the output stream
func (f *defaultFormatter) Printf(out io.Writer, format string, a ...interface{}) {
	fmt.Fprintf(out, format, a...)
}

// Println prints a message to the output stream
func (f *defaultFormatter) Println(out io.Writer, format string, a ...interface{}) {
	fmt.Fprintf(out, format+"\n", a...)
}
