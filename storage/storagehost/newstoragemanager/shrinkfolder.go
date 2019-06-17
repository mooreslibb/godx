// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package newstoragemanager

import (
	"errors"
	"fmt"
	"github.com/DxChainNetwork/godx/storage"
	"sync"

	"github.com/DxChainNetwork/godx/common/writeaheadlog"
	"github.com/DxChainNetwork/godx/rlp"
	"github.com/syndtr/goleveldb/leveldb"
)

// shrinkFolderUpdate shrinks the folder to the target size.
// The processing of shrinkFolderUpdates acquires an exclusive lock from the module,
// so no worry about locks in this update. The update will relocate the sectors that
// needs to be relocated because the folder shrinks.
type (
	shrinkFolderUpdate struct {
		folderPath string

		prevNumSectors uint64

		targetNumSectors uint64

		targetFolder *storageFolder

		relocates []sectorRelocation

		folders map[folderID]*storageFolder

		txn   *writeaheadlog.Transaction
		batch *leveldb.Batch
	}

	shrinkFolderInitPersist struct {
		FolderPath       string
		PrevNumSectors   uint64
		TargetNumSectors uint64
	}

	sectorRelocation struct {
		ID           sectorID
		PrevLocation sectorLocation
		NewLocation  sectorLocation
	}

	sectorLocation struct {
		FolderID folderID
		Index    uint64
		Count    uint64
	}
)

func (sm *storageManager) shrinkFolder(folderPath string, targetSize uint64) (err error) {
	if err = sm.tm.Add(); err != nil {
		return errStopped
	}
	defer sm.tm.Done()

	update := createShrinkFolderUpdate(folderPath, targetSize)
	// TODO: implement this
}

func createShrinkFolderUpdate(folderPath string, targetSize uint64) (update *shrinkFolderUpdate) {
	update = &shrinkFolderUpdate{
		folderPath:       folderPath,
		targetNumSectors: sizeToNumSectors(targetSize),
		folders:          make(map[folderID]*storageFolder),
	}
	return update
}

func (update *shrinkFolderUpdate) str() (s string) {
	s = fmt.Sprintf("shrink folder [%v] to %v bytes", update.folderPath, numSectorsToSize(update.targetNumSectors))
	return
}

func (update *shrinkFolderUpdate) recordIntent(manager *storageManager) (err error) {
	manager.lock.Lock()
	defer func() {
		if err != nil {
			manager.lock.Unlock()
		}
	}()
	if err = manager.folders.validateShrink(update.folderPath, update.targetNumSectors); err != nil {
		return
	}
	// get the storage folder from folders
	update.targetFolder, err = manager.folders.getWithoutLock(update.folderPath)
	if err != nil {
		return err
	}
	update.prevNumSectors = update.targetFolder.numSectors

	// record the intent
	persist := shrinkFolderInitPersist{
		FolderPath:       update.folderPath,
		PrevNumSectors:   update.targetFolder.numSectors,
		TargetNumSectors: update.targetNumSectors,
	}
	b, err := rlp.EncodeToBytes(persist)
	if err != nil {
		return err
	}
	op := writeaheadlog.Operation{
		Name: opNameShrinkFolder,
		Data: b,
	}
	if update.txn, err = manager.wal.NewTransaction([]writeaheadlog.Operation{op}); err != nil {
		return err
	}
	return
}

func (update *shrinkFolderUpdate) prepare(manager *storageManager, target uint8) (err error) {
	update.batch = manager.db.newBatch()
	switch target {
	case targetNormal:
		err = update.prepareNormal(manager)
	case targetRecoverCommitted:
		err = update.prepareCommitted(manager)
	default:
		err = errors.New("invalid target")
	}
	return
}

func (update *shrinkFolderUpdate) process(manager *storageManager, target uint8) (err error) {
	switch target {
	case targetNormal:
		err = update.processNormal(manager)
	case targetRecoverCommitted:
		err = update.processCommitted(manager)
	default:
		err = errors.New("invalid target")
	}
	return
}

func (update *shrinkFolderUpdate) prepareNormal(manager *storageManager) (err error) {
	var once sync.Once
	update.targetFolder.status = folderUnavailable
	update.targetFolder.numSectors = update.targetNumSectors
	update.folders[update.targetFolder.id] = update.targetFolder
	// get all related sectors
	ids := manager.db.getAllSectorsIDsFromFolder(update.targetFolder.id)
	for _, id := range ids {
		s, err := manager.db.getSector(id)
		if err != nil {
			return err
		}
		if s.index < update.targetNumSectors {
			// No need to update the s
			continue
		}
		// s needs to be relocated. First try to relocate the s in the same folder
		// If the folder is full, then try to relocate the s to other folders
		index, err := update.targetFolder.freeSectorIndex()
		var relocatedFolder *storageFolder
		if err == nil {
			// the s can be filled in
			relocatedFolder = update.targetFolder
		} else if err == errFolderAlreadyFull {
			relocatedFolder, index, err = manager.folders.selectFolderToAdd()
			if err != nil {
				return
			}
		} else {
			return fmt.Errorf("cannot get free s index")
		}
		if _, exist := update.folders[relocatedFolder.id]; !exist {
			update.folders[relocatedFolder.id] = relocatedFolder
		}
		// Update the memory
		if err = update.targetFolder.setFreeSectorSlot(s.index); err != nil {
			return
		}
		if err = relocatedFolder.setUsedSectorSlot(index); err != nil {
			_ = update.targetFolder.setFreeSectorSlot(s.index)
			return
		}
		relocate := sectorRelocation{
			ID: s.id,
			PrevLocation: sectorLocation{
				s.folderID, s.index, s.count,
			},
			NewLocation: sectorLocation{
				relocatedFolder.id, index, s.count,
			},
		}
		update.relocates = append(update.relocates, relocate)
		// Append the transaction
		once.Do(func() {
			if <-update.txn.InitComplete; update.txn.InitErr != nil {
				err = update.txn.InitErr
				return
			}
		})
		if err != nil {
			return
		}
		b, err := rlp.EncodeToBytes(relocate)
		if err != nil {
			return err
		}
		op := writeaheadlog.Operation{
			Name: opNameRelocateSector,
			Data: b,
		}
		if err = <-update.txn.Append([]writeaheadlog.Operation{op}); err != nil {
			return
		}
		// Append the database batch
		newSector := &sector{
			id:       s.id,
			folderID: relocatedFolder.id,
			index:    index,
			count:    s.count,
		}
		update.batch = manager.db.deleteFolderSectorToBatch(update.batch, s.folderID, s.id)
		update.batch, err = manager.db.saveSectorToBatch(update.batch, newSector, true)
		if err != nil {
			return
		}
		update.batch, err = manager.db.saveStorageFolderToBatch(update.batch, update.targetFolder)
		if err != nil {
			return
		}
		if relocatedFolder != update.targetFolder {
			update.batch, err = manager.db.saveStorageFolderToBatch(update.batch, relocatedFolder)
			if err != nil {
				return
			}
		}
	}
	return
}

func (update *shrinkFolderUpdate) prepareCommitted(manager *storageManager) (err error) {
	return
}

func (update *shrinkFolderUpdate) processNormal(manager *storageManager) (err error) {
	// commit the transaction
	if err = <-update.txn.Commit(); err != nil {
		return err
	}
	// write the data from prevLocation to afterLocation
	b := make([]byte, storage.SectorSize)
	for _, relocate := range update.relocates {
		// read data
		prevIndex := relocate.PrevLocation.Index
		n, err := update.targetFolder.dataFile.ReadAt(b, int64(prevIndex*storage.SectorSize))
		if err != nil || uint64(n) != storage.SectorSize {
			return fmt.Errorf("not read full sector")
		}
		// write data
		targetFolder, exist := update.folders[relocate.NewLocation.FolderID]
		if !exist {
			return fmt.Errorf("folder not in folders")
		}
		newIndex := relocate.NewLocation.Index
		n, err = targetFolder.dataFile.WriteAt(b, int64(newIndex))
	}
	// write the db batch
	if err = manager.db.writeBatch(update.batch); err != nil {
		return err
	}
	if err = update.targetFolder.dataFile.Truncate(int64(numSectorsToSize(update.targetNumSectors))); err != nil {
		return err
	}
	return
}

func (update *shrinkFolderUpdate) processCommitted(manager *storageManager) (err error) {
	return errRevert
}

func (update *shrinkFolderUpdate) release(manager *storageManager, upErr *updateError) (err error) {
	defer func() {
		update.targetFolder.status = folderAvailable
		manager.lock.Unlock()
	}()
	if upErr == nil || upErr.isNil() {
		err = update.txn.Release()
		return
	}
	if upErr.hasErrStopped() {
		upErr.processErr = nil
		upErr.prepareErr = nil
		return
	}
	if upErr.prepareErr != nil {
		// revert memory
	}
	// If any error happened in process, the last operation to truncate the file
	// must not have been processed and succeeded. So all the data are still safely stored on
	// the original file. It would be safe to revert all the relocates.
	return
}

func (update *shrinkFolderUpdate) lockResource(manager *storageManager) (err error) {
	manager.lock.Lock()
	return
}
