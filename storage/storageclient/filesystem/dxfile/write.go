package dxfile

import (
	"fmt"
	"github.com/DxChainNetwork/godx/rlp"
	"os"
)

// saveAll save all contents of a DxFile to the file.
func (df *DxFile) saveAll() error {
	var updates []dxfileUpdate
	iu, hostTableSize, err := df.createHostTableUpdate()
	if err != nil {
		return err
	}
	updates = append(updates, iu)
	pagesHostTable := hostTableSize / PageSize
	if hostTableSize % PageSize != 0{
		pagesHostTable++
	}
	df.metadata.SegmentOffset = df.metadata.HostTableOffset + PageSize * pagesHostTable
	segmentPersistSize := PageSize * segmentPersistNumPages(df.metadata.NumSectors)

	for i := range df.segments {
		offset := df.metadata.SegmentOffset + uint64(i) * segmentPersistSize
		update, err := df.createSegmentUpdate(uint64(i), offset)
		if err != nil {
			return err
		}
		updates = append(updates, update)
	}
	iu, err = df.createMetadataUpdate()
	if err != nil {
		return err
	}
	updates = append(updates, iu)
	return df.applyUpdates(updates)
}

// saveHostTableUpdate save the host table as well as the metadata
func (df *DxFile) saveHostTableUpdate() error {
	var updates []dxfileUpdate

	iu, hostTableSize, err := df.createHostTableUpdate()
	if err != nil {
		return err
	}
	updates = append(updates, iu)

	shiftNeeded := hostTableSize > df.metadata.SegmentOffset - df.metadata.HostTableOffset
	if shiftNeeded {
		shiftUpdates, err := df.segmentShift(hostTableSize)
		if err != nil {
			return err
		}
		updates = append(updates, shiftUpdates...)
	}
	iu, err = df.createMetadataUpdate()
	if err != nil {
		return err
	}
	updates = append(updates, iu)
	return df.applyUpdates(updates)
}

// segmentShift shift the first segment in persist file to the end of the persist file.
// Return the corresponding update and the underlying error.
func (df *DxFile) segmentShift(targetHostTableSize uint64) ([]dxfileUpdate, error) {
	f, err := os.OpenFile(df.filePath, os.O_RDONLY, 0777)
	if err != nil {
		return nil, fmt.Errorf("failed to open the file %v: %v", df.filePath, err)
	}
	defer f.Close()

	shiftOffset, numSegToShift, segmentOffsetDiff := df.shiftOffset(targetHostTableSize)
	fmt.Printf("shift: %d num: %d\n", shiftOffset, numSegToShift)
	prevOffset := df.metadata.SegmentOffset
	segmentSize := PageSize * segmentPersistNumPages(df.metadata.NumSectors)

	var updates []dxfileUpdate
	for i := 0; uint64(i) < numSegToShift; i++ {
		seg, err := df.readSegment(f, prevOffset)
		if err != nil {
			return nil, err
		}
		newOffset := prevOffset + shiftOffset
		iu, err := df.createSegmentUpdate(seg.index, newOffset)
		if err != nil {
			return nil, fmt.Errorf("failed to create segment update: %v", err)
		}
		updates = append(updates, iu)
		prevOffset += segmentSize
	}
	df.metadata.SegmentOffset += segmentOffsetDiff
	return updates, nil
}

// shiftOffset calculate for shift operation. return three offsets:
// 1. The size of shift
// 2. The number to segments to shift
// 3. Difference between new and old segment offset
func (df *DxFile) shiftOffset(targetHostTableSize uint64) (uint64, uint64, uint64) {
	if targetHostTableSize < df.metadata.SegmentOffset - df.metadata.HostTableOffset {
		return 0, 0, 0
	}
	numPagePerSeg := segmentPersistNumPages(df.metadata.NumSectors)
	sizePerSeg := PageSize * numPagePerSeg
	prevHostTableSize := df.metadata.SegmentOffset - df.metadata.HostTableOffset
	numShiftSeg := (targetHostTableSize - prevHostTableSize) / sizePerSeg
	if (targetHostTableSize - prevHostTableSize) % sizePerSeg != 0 {
		numShiftSeg++
	}
	numSeg := uint64(len(df.segments))
	var numSegToShift = numShiftSeg
	if numSegToShift > uint64(numSeg) {
		return numSegToShift * sizePerSeg, numSeg, numShiftSeg * sizePerSeg
	}
	return numSeg * sizePerSeg, numSegToShift, sizePerSeg
}

// saveMetadataUpdate create and save the metadata, return the list of update and underlying error
func (df *DxFile) saveMetadataUpdate() error{
	iu, err := df.createMetadataUpdate()
	if err != nil {
		return err
	}
	return df.applyUpdates([]dxfileUpdate{iu})
}

// createMetadataUpdate create an insert update for metadata
func (df *DxFile) createMetadataUpdate() (dxfileUpdate, error) {
	metaBytes, err := rlp.EncodeToBytes(df.metadata)
	if err != nil {
		return nil, err
	}
	if len(metaBytes) > PageSize {
		// This shall never happen
		return nil, fmt.Errorf("metadata should not have length larger than %v", PageSize)
	}
	return df.createInsertUpdate(0, metaBytes)
}

// createHostTableUpdate create a hostTable update. Return the insertUpdate, size of hostTable bytes
// and the error
func (df *DxFile) createHostTableUpdate() (dxfileUpdate, uint64, error) {
	hostTableBytes, err := rlp.EncodeToBytes(df.hostTable)
	if err != nil {
		return nil, 0, err
	}
	iu, err := df.createInsertUpdate(df.metadata.HostTableOffset, hostTableBytes)
	if err != nil {
		return nil, 0, err
	}
	return iu, uint64(len(hostTableBytes)), nil
}

// createSegmentShiftUpdate create an segment update
func (df *DxFile) createSegmentUpdate(segmentIndex uint64, offset uint64) (dxfileUpdate, error) {
	if segmentIndex > uint64(len(df.segments)) {
		return nil, fmt.Errorf("unexpected index: %d", segmentIndex)
	}
	segment := df.segments[segmentIndex]
	if segment.index != segmentIndex {
		return nil, fmt.Errorf("data corrupted: segment index not align: %d != %d", segment.index, segmentIndex)
	}
	segment.index = segmentIndex
	segment.offset = offset
	segBytes, err := rlp.EncodeToBytes(segment)
	if err != nil {
		return nil, fmt.Errorf("cannot enocde segment: %+v", segment)
	}
	if limit := PageSize * segmentPersistNumPages(df.metadata.NumSectors); uint64(len(segBytes)) > limit {
		return nil, fmt.Errorf("segment bytes exceed limit: %d > %d", len(segBytes), limit)
	}
	if int64(offset) < 0 {
		return nil, fmt.Errorf("uint64 overflow: %v", int64(offset))
	}
	return df.createInsertUpdate(offset, segBytes)
}
