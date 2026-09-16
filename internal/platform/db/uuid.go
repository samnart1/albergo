package db

import (
	"fmt"
	"uuid"

	"github.com/jackc/pgx/v5/pgtype"
)

// pgx
type pgUUID uuid.UUID

func (u *pgUUID) ScanUUID(v pgtype.UUID) error {
	if !v.Valid {
		return fmt.Errorf("cannot scan NULL into uuid.UUID")
	}
	*u = pgUUID(v.Bytes)
	return nil
}

func (u pgUUID) UUIDValue() (pgtype.UUID, error) {
	return pgtype.UUID{Bytes: [16]byte(u), Valid: true}, nil
}

type uuidCodec struct {
	pgtype.UUIDCodec
}

func (uuidCodec) DecodeValue(m *pgtype.Map, oid uint32, format int16, src []byte) (any, error) {
	if src == nil {
		return nil, nil
	}

	var target uuid.UUID
	plan := m.PlanScan(oid, format, &target)
	if plan == nil {
		return nil, fmt.Errorf("no scan plan for uuid")
	}
	if err := plan.Scan(src, &target); err != nil {
		return nil, err
	}
	return target, nil
}

type wrapUUIDEncodePlan struct{ next pgtype.EncodePlan }

func (p *wrapUUIDEncodePlan) SetNext(next pgtype.EncodePlan) { p.next = next }

func (p *wrapUUIDEncodePlan) Encode(value any, buf []byte) ([]byte, error) {
	return p.next.Encode(pgUUID(value.(uuid.UUID)), buf)
}

func tryWrapUUIDEncodePlan(value any) (pgtype.WrappedEncodePlanNextSetter, any, bool) {
	if v, ok := value.(uuid.UUID); ok {
		return &wrapUUIDEncodePlan{}, pgUUID(v), true
	}
	return nil, nil, false
}

type wrapUUIDScanPlan struct{ next pgtype.ScanPlan }

func (p *wrapUUIDScanPlan) SetNext(next pgtype.ScanPlan) { p.next = next }

func (p *wrapUUIDScanPlan) Scan(src []byte, dst any) error {
	return p.next.Scan(src, (*pgUUID)(dst.(*uuid.UUID)))
}

func tryWrapUUIDScanPlan(target any) (pgtype.WrappedScanPlanNextSetter, any, bool) {
	if v, ok := target.(*uuid.UUID); ok {
		return &wrapUUIDScanPlan{}, (*pgUUID)(v), true
	}
	return nil, nil, false
}

func registerUUID(m *pgtype.Map) {
	m.TryWrapEncodePlanFuncs = append([]pgtype.TryWrapEncodePlanFunc{tryWrapUUIDEncodePlan}, m.TryWrapEncodePlanFuncs...)
	m.TryWrapScanPlanFuncs = append([]pgtype.TryWrapScanPlanFunc{tryWrapUUIDScanPlan}, m.TryWrapScanPlanFuncs...)
	m.RegisterType(&pgtype.Type{Name: "uuid", OID: pgtype.UUIDOID, Codec: uuidCodec{}})
}
