package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/fatih/color"
	"github.com/invopop/gobl/uuid"
	"github.com/invopop/invocli/pkg/sequence"
	"github.com/manifoldco/promptui"
)

// Series represents a sequential series of numbers.

func listSeries(seq sequence.Client) {
	sc := new(sequence.SeriesCollection)
	if err := seq.List(context.Background(), sc); err != nil {
		color.HiRed(err.Error())
		return
	}
	d, _ := json.MarshalIndent(sc, "", "    ")
	fmt.Println(string(d))
}

func createSeries(seq sequence.Client) {
	var err error
	s := new(sequence.Series)
	s.ID = uuid.NewV4().String()

	p := promptui.Prompt{
		Label: "Series Name",
	}
	s.Name, err = p.Run()
	if err != nil {
		fmt.Printf("prompt failed: %v\n", err.Error())
		return
	}
	p = promptui.Prompt{
		Label: "Series Prefix",
	}
	s.Prefix, err = p.Run()
	if err != nil {
		fmt.Printf("prompt failed: %v\n", err.Error())
		return
	}
	p = promptui.Prompt{
		Label: "Series Suffix",
	}
	s.Suffix, err = p.Run()
	if err != nil {
		fmt.Printf("prompt failed: %v\n", err.Error())
		return
	}
	ps := promptui.Select{
		Label: "Zero Padding",
		Items: []string{"0", "1", "2", "3", "4", "5", "6", "7", "8"},
	}
	_, si, err := ps.Run()
	if err != nil {
		fmt.Printf("prompt select failed: %v\n", err.Error())
		return
	}
	i, _ := strconv.ParseInt(si, 32, 10)
	s.Padding = int32(i)

	if err := seq.Create(context.Background(), s); err != nil {
		color.HiRed(err.Error())
		return
	}

	d, _ := json.MarshalIndent(s, "", "    ")
	fmt.Println(string(d))
}

func nextSeriesEntry(seq sequence.Client, seriesID string) {
	se := new(sequence.Entry)
	se.ID = uuid.NewV1().String()

	if err := seq.CreateEntry(context.Background(), seriesID, se); err != nil {
		color.HiRed(err.Error())
		return
	}

	d, _ := json.MarshalIndent(se, "", "    ")
	fmt.Println(string(d))
}
