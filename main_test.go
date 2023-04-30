package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHTMLTableTo2D_A(t *testing.T) {
	rows := [][]Cell{
		{
			{rowspan: 1, colspan: 1, text: "A"},
			{rowspan: 1, colspan: 1, text: "B"},
		},
		{
			{rowspan: 2, colspan: 1, text: "C"},
			{rowspan: 1, colspan: 1, text: "D"},
		},
		{
			{rowspan: 1, colspan: 1, text: "E"},
			{rowspan: 1, colspan: 1, text: "F"},
		},
		{
			{rowspan: 1, colspan: 1, text: "G"},
			{rowspan: 1, colspan: 1, text: "H"},
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
			{rowspan: 1, colspan: 1, text: "A"},
			{rowspan: 1, colspan: 1, text: "B"},
		},
		{
			{rowspan: 2, colspan: 1, text: "C"},
			{rowspan: 2, colspan: 1, text: "D"},
		},
		{
			{rowspan: 1, colspan: 1, text: "E"},
			{rowspan: 1, colspan: 1, text: "F"},
		},
		{
			{rowspan: 1, colspan: 1, text: "G"},
			{rowspan: 1, colspan: 1, text: "H"},
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
			{rowspan: 3, colspan: 1, text: "A"},
			{rowspan: 0, colspan: 1, text: "B"},
			{rowspan: 1, colspan: 1, text: "C"},
			{rowspan: 1, colspan: 2, text: "D"},
		},
		{
			{rowspan: 1, colspan: 0, text: "E"},
		},
	}

	table := htmlTableTo2D(rows)

	expected := [][]string{
		{"A", "B", "C", "D"},
		{"A", "B", "E", "E"},
	}

	assert.Equal(t, expected, table)
}
