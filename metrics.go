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
	"time"

	"github.com/kchristidis/island/chaincode/schema"
	"github.com/kchristidis/island/stats"
)

func metrics() error {
	msg := fmt.Sprint("main • time to collect & print the results...")
	fmt.Fprintln(writer, msg)

	msg = fmt.Sprint("main • closing donestatsc...")
	fmt.Fprintln(writer, msg)
	close(doneStatsC)
	wg3.Wait()

	// Create the output dir if it doesn't exist already
	if _, err := os.Stat(OutputDir); os.IsNotExist(err) {
		if err := os.Mkdir(OutputDir, 755); err != nil {
			return err
		}
	}

	tranFile, err := os.Create(filepath.Join(OutputDir, fmt.Sprintf("%s-%s", outputPrefix, OutputTran)))
	if err != nil {
		return err
	}
	defer tranFile.Close()

	tranWriter := csv.NewWriter(tranFile)
	if err := tranWriter.Write([]string{"tx_id", "latency_ms", "tx_type", "attempt", "tx_status"}); err != nil {
		return err
	}
	defer func() error {
		tranWriter.Flush()
		if err := tranWriter.Error(); err != nil {
			return err
		}
		return nil
	}()

	blockFile, err := os.Create(filepath.Join(OutputDir, fmt.Sprintf("%s-%s", outputPrefix, OutputBlock)))
	if err != nil {
		return err
	}
	defer blockFile.Close()

	blockWriter := csv.NewWriter(blockFile)
	if err := blockWriter.Write([]string{"block_num", "size_kib"}); err != nil {
		return err
	}
	defer func() error {
		blockWriter.Flush()
		if err := blockWriter.Error(); err != nil {
			return err
		}
		return nil
	}()

	slotFile, err := os.Create(filepath.Join(OutputDir, fmt.Sprintf("%s-%s", outputPrefix, OutputSlot)))
	if err != nil {
		return err
	}
	defer slotFile.Close()

	slotWriter := csv.NewWriter(slotFile)
	if err := slotWriter.Write([]string{"slot_num",
		"bfg_qty_kwh", "bfg_ppu_c_per_kWh)", // bfg = bought from grid
		"stg_qty_kwh", "stg_ppu_c_per_kWh)", // stg = sold to grid
		"dmi_qty_kwh", "dmi_ppu_c_per_kWh)", // dmi = demand met internally
		"late_cnt_all", "late_cnt_buy", "late_cnt_sell",
		"late_decrs",
		"prob_iters", "prob_marshals",
		"prob_decrs", "prob_bids",
		"prob_keys", "prob_gets", "prob_puts"}); err != nil {
		return err
	}
	defer func() error {
		slotWriter.Flush()
		if err := slotWriter.Error(); err != nil {
			return err
		}
		return nil
	}()

	if schema.StagingLevel <= schema.Debug {
		println()
		fmt.Fprintln(writer, "transaction stats")
	}

	for _, tx := range stats.TransactionStats {
		idNum, _ := strconv.Atoi(tx.ID)
		idVal := fmt.Sprintf("%012d", idNum)
		latVal := fmt.Sprintf("%d", tx.LatencyInMillis)
		attVal := fmt.Sprintf("%d", tx.Attempt)
		msg := fmt.Sprintf("[event_id: %s]"+
			"\tlatency:%s ms"+
			"\t\ttype:%s"+
			"\t\tattempt:%s"+
			"\t\tstatus:%s",
			idVal,
			latVal,
			tx.Type,
			attVal,
			tx.Status,
		)
		if schema.StagingLevel <= schema.Debug {
			fmt.Fprintln(writer, msg)
		}
		if err := tranWriter.Write([]string{idVal, latVal, tx.Type, attVal, tx.Status}); err != nil {
			return err
		}
	}

	if schema.StagingLevel <= schema.Debug {
		println()
		fmt.Fprintln(writer, "block stats")
	}

	for _, block := range stats.BlockStats {
		numVal := fmt.Sprintf("%012d", block.Number)
		sizeVal := fmt.Sprintf("%.1f", block.Size) // ATTN: This is the size in KiB
		msg := fmt.Sprintf("[block: %s]"+
			"\t%s KiB",
			numVal,
			sizeVal,
		)
		if schema.StagingLevel <= schema.Debug {
			fmt.Fprintln(writer, msg)
		}
		if err := blockWriter.Write([]string{numVal, sizeVal}); err != nil {
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

		if err := slotWriter.Write([]string{slotVal,
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

	msg = fmt.Sprintf("main • run completed in %s", time.Now().Sub(timeStart))
	fmt.Fprintln(writer, msg)

	return nil
}
