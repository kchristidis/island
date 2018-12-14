package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strconv"

	"github.com/golang/protobuf/proto"
	"github.com/kchristidis/island/chaincode/schema"
	"github.com/kchristidis/island/stats"
)

func metrics() error {
	msg := fmt.Sprint("main • time to collect & print the results...")
	fmt.Fprintln(writer, msg)

	msg = fmt.Sprint("main • closing donestatsc...")
	fmt.Fprintln(writer, msg)
	close(donestatsc)
	wg3.Wait()

	// Create the output dir if it doesn't exist already
	if _, err := os.Stat(OutputDir); os.IsNotExist(err) {
		if err := os.Mkdir(OutputDir, 755); err != nil {
			return err
		}
	}

	tranfile, err := os.Create(filepath.Join(OutputDir, fmt.Sprintf("%s-%s", outputprefix, OutputTran)))
	if err != nil {
		return err
	}
	defer tranfile.Close()

	tranwriter := csv.NewWriter(tranfile)
	if err := tranwriter.Write([]string{"tx ID", "latency (ms)", "tx type", "tx status"}); err != nil {
		return err
	}
	defer func() error {
		tranwriter.Flush()
		if err := tranwriter.Error(); err != nil {
			return err
		}
		return nil
	}()

	blockfile, err := os.Create(filepath.Join(OutputDir, fmt.Sprintf("%s-%s", outputprefix, OutputBlock)))
	if err != nil {
		return err
	}
	defer blockfile.Close()

	blockwriter := csv.NewWriter(blockfile)
	if err := blockwriter.Write([]string{"block num", "filesize (KiB)"}); err != nil {
		return err
	}
	defer func() error {
		blockwriter.Flush()
		if err := blockwriter.Error(); err != nil {
			return err
		}
		return nil
	}()

	slotfile, err := os.Create(filepath.Join(OutputDir, fmt.Sprintf("%s-%s", outputprefix, OutputSlot)))
	if err != nil {
		return err
	}
	defer slotfile.Close()

	slotwriter := csv.NewWriter(slotfile)
	if err := slotwriter.Write([]string{"slot num",
		"bfg qty (kWh)", "bfg ppu (ç/kWh)", // bfg = bought from grid
		"stg qty (kWh)", "stg ppu (ç/kWh)", // stg = sold to grid
		"dmi qty (kWh)", "dmi ppu (ç/kWh)", // dmi = demand met internally
		"late cnt (all)", "late cnt (buy)", "late cnt (sell)",
		"late decrs",
		"prob iters", "prob marshals",
		"prob decrs", "prob bids",
		"prob keys", "prob gets", "prob puts"}); err != nil {
		return err
	}
	defer func() error {
		slotwriter.Flush()
		if err := slotwriter.Error(); err != nil {
			return err
		}
		return nil
	}()

	println()

	fmt.Fprintln(writer, "transaction stats")
	for _, tx := range stats.TransactionStats {
		idNum, _ := strconv.Atoi(tx.ID)
		idVal := fmt.Sprintf("%012d", idNum)
		latVal := fmt.Sprintf("%d", tx.LatencyInMillis)
		msg := fmt.Sprintf("[event_id: %s]"+
			"\tlatency:%s ms"+
			"\t\ttype:%s"+
			"\t\tstatus:%s",
			idVal,
			latVal,
			tx.Type,
			tx.Status,
		)
		fmt.Fprintln(writer, msg)
		if err := tranwriter.Write([]string{idVal, latVal, tx.Type, tx.Status}); err != nil {
			return err
		}
	}

	println()

	fmt.Fprintln(writer, "block stats")

	querier := sdkctx.LedgerClient
	resp, err := querier.QueryInfo()
	if err != nil {
		msg := fmt.Sprintf("main • cannot query ledger: %s", err.Error())
		fmt.Fprintln(writer, msg)
		return err
	}

	height := resp.BCI.GetHeight()

	for i := uint64(0); i < height; i++ {
		block, err := querier.QueryBlock(i)
		if err != nil {
			msg := fmt.Sprintf("main • can get block %d's size: %s", i, err.Error())
			fmt.Fprintln(writer, msg)
			return err
		}
		blockB, err := proto.Marshal(block)
		if err != nil {
			msg := fmt.Sprintf("main • cannot marshal block %d: %s", i, err.Error())
			fmt.Fprintln(writer, msg)
			return err
		}
		numVal := fmt.Sprintf("%012d", i)
		sizeVal := fmt.Sprintf("%.1f", float32(len(blockB))/1024) // ATTN: This is the size in KiB
		msg := fmt.Sprintf("[block: %s]"+
			"\t%s KiB", numVal, sizeVal,
		)
		fmt.Fprintln(writer, msg)
		if err := blockwriter.Write([]string{numVal, sizeVal}); err != nil {
			return err
		}
	}

	println()

	var (
		metricsOutputVal schema.MetricsOutput
		respB            []byte
	)

	args := schema.OpContextInput{
		EventID: strconv.Itoa(rand.Intn(1E12)),
		Action:  "metrics",
	}
	if respB, err = sdkctx.Query(args); err != nil {
		return err
	}

	if err := json.Unmarshal(respB, &metricsOutputVal); err != nil {
		msg := fmt.Sprintf("main • cannot unmarshal returned response: %s", err.Error())
		fmt.Fprintln(writer, msg)
	}

	msg = fmt.Sprintf("main • cleared slot stats")
	fmt.Fprintln(writer, msg)
	// We decrement LargestSlotSeen by 1 because we care about the
	// *cleared* slot, i.e. those slots where we had a MarkEnd call.
	for i := 0; i <= stats.LargestSlotSeen-1; i++ {
		slotVal := fmt.Sprintf("%012d", stats.SlotStats[i].Number)
		bfgQtyVal := fmt.Sprintf("%.3f", stats.SlotStats[i].EnergyUse)
		bfgPpuVal := fmt.Sprintf("%.3f", stats.SlotStats[i].PricePaid)
		stgQtyVal := fmt.Sprintf("%.3f", stats.SlotStats[i].EnergyGen)
		stgPpuVal := fmt.Sprintf("%.3f", stats.SlotStats[i].PriceSold)
		dmiQtyVal := fmt.Sprintf("%.3f", stats.SlotStats[i].EnergyTraded)
		dmiPpuVal := fmt.Sprintf("%.3f", stats.SlotStats[i].PriceTraded)
		lateAllVal := fmt.Sprintf("%d", metricsOutputVal.LateTXsCount[i])
		lateBuyVal := fmt.Sprintf("%d", metricsOutputVal.LateBuysCount[i])
		lateSellVal := fmt.Sprintf("%d", metricsOutputVal.LateSellsCount[i])
		lateDecrVal := fmt.Sprintf("%d", metricsOutputVal.LateDecryptsCount[i])
		probIterVal := fmt.Sprintf("%d", metricsOutputVal.ProblematicIterCount[i])
		probMarVal := fmt.Sprintf("%d", metricsOutputVal.ProblematicMarshalCount[i])
		probDecrVal := fmt.Sprintf("%d", metricsOutputVal.ProblematicDecryptCount[i])
		probBidCalcVal := fmt.Sprintf("%d", metricsOutputVal.ProblematicBidCalcCount[i])
		probKeyVal := fmt.Sprintf("%d", metricsOutputVal.ProblematicKeyCount[i])
		probGetVal := fmt.Sprintf("%d", metricsOutputVal.ProblematicGetStateCount[i])
		probPutVal := fmt.Sprintf("%d", metricsOutputVal.ProblematicPutStateCount[i])
		msg := fmt.Sprintf("[slot: %s]"+
			"\t%s kWh bought from the grid @ %s ç/kWh"+
			"\t\t%s kWh sold to grid @ %s ç/kWh"+
			"\t\t%s kWh of demand met internally @ %s ç/kWh"+
			"\t\t%s late transactions (total)"+
			"\t\t%s late buy transactions"+
			"\t\t%s late sell transactions"+
			"\t\t%s late decryptions"+
			"\t\t%s problematic iterations"+
			"\t\t%s problematic de/serializations"+
			"\t\t%s problematic decrypts"+
			"\t\t%s problematic bid calculations"+
			"\t\t%s problematic key creations"+
			"\t\t%s problematic get states"+
			"\t\t%s problematic put states",
			slotVal,
			bfgQtyVal, bfgPpuVal,
			stgQtyVal, stgPpuVal,
			dmiQtyVal, dmiPpuVal,
			lateAllVal, lateBuyVal, lateSellVal,
			lateDecrVal,
			probIterVal, probMarVal, probDecrVal, probBidCalcVal,
			probKeyVal, probGetVal, probPutVal,
		)
		fmt.Fprintln(writer, msg)

		if err := slotwriter.Write([]string{slotVal,
			bfgQtyVal, bfgPpuVal,
			stgQtyVal, stgPpuVal,
			dmiQtyVal, dmiPpuVal,
			lateAllVal, lateBuyVal, lateSellVal,
			lateDecrVal,
			probIterVal, probMarVal, probDecrVal,
			probKeyVal, probGetVal, probPutVal}); err != nil {
			return err
		}
	}

	println()

	msg = fmt.Sprintf("main • number of goroutines still running: %d", runtime.NumGoroutine())
	fmt.Fprintln(writer, msg)

	return nil
}
