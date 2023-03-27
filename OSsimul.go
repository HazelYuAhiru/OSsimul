package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// semaphore structure as resource manager
type semaphore struct {
	isFree []bool
	semC   chan int
}

func (s *semaphore) Acquire() int {
	for i := 0; i < len(s.isFree); i++ {
		if s.isFree[i] {
			s.isFree[i] = false
			s.semC <- i
			return i
		}
	}
	return -1
}

func (s *semaphore) Release() {
	freeInd := <-s.semC
	s.isFree[freeInd] = true
}

// Disk
type Disk struct {
	diskID   int
	sector   []string
	nextFree int
}

func Dconstruct(id int) *Disk {
	d := new(Disk)
	d.diskID = id
	d.sector = make([]string, 2048)
	d.nextFree = 0

	return d
}

func write(d *Disk, numsec int, data string) {
	time.Sleep(80)
	d.sector[numsec] = data
	d.nextFree += 1
}

func read(d *Disk, secnum int) string {
	return d.sector[secnum]
}

// diskManager
type DiskManager struct {
	numDisk       int
	diskList      []*Disk
	diskDirectory map[string]FileInfo
	diskSem       semaphore
}

func DMconstruct(disknum int) *DiskManager {
	d := new(DiskManager)
	d.numDisk = disknum
	d.diskDirectory = make(map[string]FileInfo)
	d.diskSem = semaphore{semC: make(chan int, disknum)}
	for i := 0; i < disknum; i++ {
		d.diskSem.isFree = append(d.diskSem.isFree, true)
	}

	for i := 0; i < disknum; i++ {
		newDisk := Dconstruct(i)
		d.diskList = append(d.diskList, newDisk)
	}

	return d
}

// fileinfo and directory manager
type FileInfo struct {
	diskNumber     int
	startingSector int
	fileLength     int
}

// printer
type Printer struct {
	outFileName string
	printerID   int
}

func Pconstruct(id int) *Printer {
	p := new(Printer)
	p.printerID = id
	p.outFileName = "PRINTER" + strconv.Itoa(id)

	return p
}

func print(p *Printer, data string) {
	time.Sleep(275)
	f, err := os.OpenFile(p.outFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Println("error in open file before write")
	} else {
		defer f.Close()
		if _, err := f.WriteString(data + "\n"); err != nil {
			fmt.Println("error in write to file")
		}
	}
}

// printerManager
type PrinterManager struct {
	numPrinter  int
	printerList []*Printer
	printerSem  semaphore
}

func PMconstruct(numPrinter int) *PrinterManager {
	pm := new(PrinterManager)
	pm.numPrinter = numPrinter
	pm.printerSem = semaphore{semC: make(chan int, numPrinter)}
	for i := 0; i < numPrinter; i++ {
		pm.printerSem.isFree = append(pm.printerSem.isFree, true)
	}

	for i := 0; i < numPrinter; i++ {
		newPrinter := Pconstruct(i)
		pm.printerList = append(pm.printerList, newPrinter)
	}

	return pm
}

// user thread
func UserThread(filename string, dManager *DiskManager, pManager *PrinterManager, wg *sync.WaitGroup) {
	defer wg.Done()

	rf, err := os.Open(filename)

	if err != nil {
		fmt.Println(err)
	} else {
		fileScanner := bufio.NewScanner(rf)
		fileScanner.Split(bufio.ScanLines)
		//track when to save
		var wdp sync.WaitGroup
		onSave := false
		//track save file and print file
		var saveFile string
		var printFile string
		var fileContent []string
		_ = saveFile
		_ = printFile

		for fileScanner.Scan() {
			curline := fileScanner.Text()
			if onSave {
				if curline == ".end" {
					onSave = false
					//save
					wdp.Add(1)
					go DiskThread(fileContent, saveFile, dManager, &wdp)
					wdp.Wait()
					//clear
					fileContent = nil

				} else {
					fileContent = append(fileContent, curline)
				}
			} else {
				if strings.Contains(curline, ".save") {
					//file info
					saveFile = curline[6:]
					onSave = true
				}
				if strings.Contains(curline, ".print") {
					printFile = curline[7:]
					//print
					wdp.Add(1)
					go PrintThread(printFile, dManager, pManager, &wdp)
					wdp.Wait()
				}
			}
		}
	}
	rf.Close()
}

func DiskThread(fileContent []string, fileName string, dManager *DiskManager, wg *sync.WaitGroup) {
	defer wg.Done()

	curDisk := dManager.diskSem.Acquire()
	startSec := dManager.diskList[curDisk].nextFree
	fileLength := len(fileContent)

	for i := 0; i < fileLength; i++ {
		write(dManager.diskList[curDisk], startSec+i, fileContent[i])
	}

	dManager.diskDirectory[fileName] = FileInfo{diskNumber: curDisk, startingSector: startSec, fileLength: fileLength}
	dManager.diskSem.Release()
}

func PrintThread(fileName string, dManager *DiskManager, pManager *PrinterManager, wg *sync.WaitGroup) {
	defer wg.Done()

	finfo, found := dManager.diskDirectory[fileName]
	if found {
		fileLength := finfo.fileLength
		filedisk := finfo.diskNumber
		startsec := finfo.startingSector

		curPrinter := pManager.printerSem.Acquire()
		for i := 0; i < fileLength; i++ {
			print(pManager.printerList[curPrinter], read(dManager.diskList[filedisk], startsec+i))
		}

		pManager.printerSem.Release()

	} else {
		fmt.Println("file not found before print!")
	}
}

func main() {
	//build: go build 141OS.go
	//run: go run 141OS.go -#ofUsers -#ofDisks -#ofPrinters

	//read from command line
	numUser, errU := strconv.Atoi(os.Args[1])
	numDisk, errD := strconv.Atoi(os.Args[2])
	numPrinter, errP := strconv.Atoi(os.Args[3])

	if errU == nil && errD == nil && errP == nil {
		numUser = numUser * -1
		numDisk = numDisk * -1
		numPrinter = numPrinter * -1

		//create managers
		dManager := DMconstruct(numDisk)
		pManager := PMconstruct(numPrinter)

		//start user threads
		var wg sync.WaitGroup
		wg.Add(numUser)

		for i := 0; i < numUser; i++ {
			go UserThread("USER"+strconv.Itoa(i), dManager, pManager, &wg)
		}

		wg.Wait()

	} else {
		fmt.Println("error in getting command line args!")
	}
}
