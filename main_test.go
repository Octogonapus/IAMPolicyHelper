package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTMLTableTo2D_A(t *testing.T) {
	rows := [][]Cell{
		{
			{Rowspan: 1, Colspan: 1, Text: "A"},
			{Rowspan: 1, Colspan: 1, Text: "B"},
		},
		{
			{Rowspan: 2, Colspan: 1, Text: "C"},
			{Rowspan: 1, Colspan: 1, Text: "D"},
		},
		{
			{Rowspan: 1, Colspan: 1, Text: "E"},
			{Rowspan: 1, Colspan: 1, Text: "F"},
		},
		{
			{Rowspan: 1, Colspan: 1, Text: "G"},
			{Rowspan: 1, Colspan: 1, Text: "H"},
		},
	}

	table := htmlTableTo2D(rows)

	expected := [][]string{
		{"A", "B", ""},
		{"C", "D", ""},
		{"C", "E", "F"},
		{"G", "H", ""},
	}

	assert.Equal(t, expected, table)
}

func TestHTMLTableTo2D_B(t *testing.T) {
	rows := [][]Cell{
		{
			{Rowspan: 1, Colspan: 1, Text: "A"},
			{Rowspan: 1, Colspan: 1, Text: "B"},
		},
		{
			{Rowspan: 2, Colspan: 1, Text: "C"},
			{Rowspan: 2, Colspan: 1, Text: "D"},
		},
		{
			{Rowspan: 1, Colspan: 1, Text: "E"},
			{Rowspan: 1, Colspan: 1, Text: "F"},
		},
		{
			{Rowspan: 1, Colspan: 1, Text: "G"},
			{Rowspan: 1, Colspan: 1, Text: "H"},
		},
	}

	table := htmlTableTo2D(rows)

	expected := [][]string{
		{"A", "B", "", ""},
		{"C", "D", "", ""},
		{"C", "D", "E", "F"},
		{"G", "H", "", ""},
	}

	assert.Equal(t, expected, table)
}

func TestHTMLTableTo2D_C(t *testing.T) {
	rows := [][]Cell{
		{
			{Rowspan: 3, Colspan: 1, Text: "A"},
			{Rowspan: 0, Colspan: 1, Text: "B"},
			{Rowspan: 1, Colspan: 1, Text: "C"},
			{Rowspan: 1, Colspan: 2, Text: "D"},
		},
		{
			{Rowspan: 1, Colspan: 0, Text: "E"},
		},
	}

	table := htmlTableTo2D(rows)

	expected := [][]string{
		{"A", "B", "C", "D"},
		{"A", "B", "E", "E"},
	}

	assert.Equal(t, expected, table)
}
