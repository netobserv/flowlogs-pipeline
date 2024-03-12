/*
 * Copyright (C) 2022 IBM, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMultiOrderedMap_AddRecord(t *testing.T) {
	mom := NewMultiOrderedMap()

	err := mom.AddRecord(5, "aaaa")
	require.NoError(t, err)
	require.Equal(t, 1, mom.Len())

	err = mom.AddRecord(5, "bb")
	require.Error(t, err)
	require.Equal(t, 1, mom.Len())

	err = mom.AddRecord(6, "bb")
	require.NoError(t, err)
	require.Equal(t, 2, mom.Len())
	assertLengthConsistency(t, mom)
}

func TestMultiOrderedMap_GetRecord(t *testing.T) {
	mom := NewMultiOrderedMap()
	record, found := mom.GetRecord(5)
	require.False(t, found)
	require.Nil(t, record)

	mustAddRecord(t, mom, 5, "aaaa")
	mustAddRecord(t, mom, 6, "bb")

	record, found = mom.GetRecord(5)
	require.True(t, found)
	require.Equal(t, "aaaa", record)
	record, found = mom.GetRecord(6)
	require.True(t, found)
	require.Equal(t, "bb", record)
	assertLengthConsistency(t, mom)
}

func TestMultiOrderedMap_RemoveRecord(t *testing.T) {
	mom := NewMultiOrderedMap()
	// Test removing non-existing key
	mom.RemoveRecord(5)

	mustAddRecord(t, mom, 5, "aaaa")

	// Test removing existing key
	mom.RemoveRecord(5)
	require.Zero(t, mom.Len())
	assertLengthConsistency(t, mom)
}

func TestMultiOrderedMap_MoveToBack(t *testing.T) {
	lengthOrder := OrderID("length")
	lexicalOrder := OrderID("lexical")
	mom := NewMultiOrderedMap(lengthOrder, lexicalOrder)

	mustAddRecord(t, mom, 5, "aaaa")
	mustAddRecord(t, mom, 6, "ddd")
	mustAddRecord(t, mom, 7, "b")
	mustAddRecord(t, mom, 8, "cc")
	assertOrder(t, mom, lengthOrder, []string{"aaaa", "ddd", "b", "cc"})
	assertOrder(t, mom, lexicalOrder, []string{"aaaa", "ddd", "b", "cc"})

	err := mom.MoveToBack(7, lengthOrder)
	require.NoError(t, err)
	err = mom.MoveToBack(6, lexicalOrder)
	require.NoError(t, err)

	assertOrder(t, mom, lengthOrder, []string{"aaaa", "ddd", "cc", "b"})
	assertOrder(t, mom, lexicalOrder, []string{"aaaa", "b", "cc", "ddd"})

	// Test invalid args
	err = mom.MoveToBack(0, lengthOrder)
	require.Error(t, err)
	err = mom.MoveToBack(5, "ERROR")
	require.Error(t, err)
	assertLengthConsistency(t, mom)
}

func TestMultiOrderedMap_MoveToFront(t *testing.T) {
	lengthOrder := OrderID("length")
	lexicalOrder := OrderID("lexical")
	mom := NewMultiOrderedMap(lengthOrder, lexicalOrder)

	mustAddRecord(t, mom, 5, "bbb")
	mustAddRecord(t, mom, 6, "aa")
	mustAddRecord(t, mom, 7, "cccc")
	mustAddRecord(t, mom, 8, "d")
	assertOrder(t, mom, lengthOrder, []string{"bbb", "aa", "cccc", "d"})
	assertOrder(t, mom, lexicalOrder, []string{"bbb", "aa", "cccc", "d"})

	err := mom.MoveToFront(7, lengthOrder)
	require.NoError(t, err)
	err = mom.MoveToFront(6, lexicalOrder)
	require.NoError(t, err)

	assertOrder(t, mom, lengthOrder, []string{"cccc", "bbb", "aa", "d"})
	assertOrder(t, mom, lexicalOrder, []string{"aa", "bbb", "cccc", "d"})

	// Test invalid args
	err = mom.MoveToFront(0, lengthOrder)
	require.Error(t, err)
	err = mom.MoveToFront(5, "ERROR")
	require.Error(t, err)
	assertLengthConsistency(t, mom)
}

func TestMultiOrderedMap_IterateFrontToBack(t *testing.T) {
	t.Run("delete all", func(t *testing.T) {
		lengthOrder := OrderID("length")
		lexicalOrder := OrderID("lexical")
		mom := NewMultiOrderedMap(lengthOrder, lexicalOrder)
		mustAddRecord(t, mom, 5, "aaaa")
		mustAddRecord(t, mom, 6, "ddd")
		mustAddRecord(t, mom, 7, "b")
		mom.IterateFrontToBack(lengthOrder, func(_ Record) (del, stop bool) {
			del = true
			return
		})
		require.Zero(t, mom.Len())
		assertLengthConsistency(t, mom)
	})
	t.Run("delete and stop", func(t *testing.T) {
		lexicalOrder := OrderID("lexical")
		mom := NewMultiOrderedMap(lexicalOrder)
		mustAddRecord(t, mom, 5, "aaaa")
		mustAddRecord(t, mom, 6, "bb")
		mustAddRecord(t, mom, 7, "ccc")
		mustAddRecord(t, mom, 8, "d")
		var actualIteratedRecords []string
		mom.IterateFrontToBack(lexicalOrder, func(record Record) (del, stop bool) {
			actualIteratedRecords = append(actualIteratedRecords, record.(string))
			if record == "bb" {
				del = true
			}
			if record == "ccc" {
				stop = true
			}
			return
		})
		require.Equal(t, []string{"aaaa", "bb", "ccc"}, actualIteratedRecords)
		assertOrder(t, mom, lexicalOrder, []string{"aaaa", "ccc", "d"})
		assertLengthConsistency(t, mom)
	})

	t.Run("invalid args", func(t *testing.T) {
		lengthOrder := OrderID("length")
		lexicalOrder := OrderID("lexical")
		mom := NewMultiOrderedMap(lengthOrder, lexicalOrder)
		require.Panics(t, func() {
			mom.IterateFrontToBack("MISSING", func(_ Record) (delete, stop bool) {
				return
			})
		})
		assertLengthConsistency(t, mom)
	})
}

func mustAddRecord(t *testing.T, mom *MultiOrderedMap, key Key, record Record) {
	t.Helper()
	require.NoError(t, mom.AddRecord(key, record))
}

func assertOrder(t *testing.T, mom *MultiOrderedMap, order OrderID, expected []string) {
	t.Helper()
	var actual []string
	mom.IterateFrontToBack(order, func(record Record) (bool, bool) {
		actual = append(actual, record.(string))
		return false, false
	})
	require.Equal(t, expected, actual)
}

func assertLengthConsistency(t *testing.T, mom *MultiOrderedMap) {
	t.Helper()
	// Verify that the length of the map and all the lists are the same and equal to MultiOrderedMap.Len()
	l := len(mom.m)
	require.Equal(t, l, mom.Len())
	for _, orderList := range mom.orders {
		require.Equal(t, l, orderList.Len())
	}
}
