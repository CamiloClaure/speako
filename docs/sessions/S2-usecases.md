# S2 - Use cases and ports

**Read first:** `AGENTS.md`, `docs/architecture.md`, everything in `api/internal/domain`
**Branch:** `be/s2-usecases`
**You own:** `api/internal/app/**` - and nothing else. No SQL, no HTTP, no vendor SDK.

## Goal

One file per user-visible operation, orchestrating domain objects through interfaces. Every use case must be
fully testable with in-memory fakes.

## Tasks

1. `app/ports`: declare exactly the interfaces the use cases need.
   - Repos: `UserRepo, ProductRepo, OrderRepo, EntitlementRepo, CreditLedger, BookingRepo, AvailabilityRepo,
     LeadRepo`
   - Services: `PaymentGateway` (`CreateCheckout(ctx, order) (url string, ref string, err error)`,
     `FetchPayment(ctx, ref)`), `FileStorage` (`SignedURL(ctx, key, ttl)`), `Mailer` (one method per message
     type, not a generic `Send(html)`)
   - Ambient: `Clock`, `IDGen`, `TokenGen`, `TxManager` (`InTx(ctx, func(ctx) error) error`)
2. Implement the use cases listed in the plan's inventory:
   `auth` (RequestMagicLink, CompleteMagicLink, SignInWithGoogle, SignOut, GetMe) ·
   `catalog` (ListProducts, GetProduct) ·
   `purchase` (StartCheckout, HandlePaymentWebhook, ListMyOrders) ·
   `materials` (ListMyMaterials, IssueDownloadURL) ·
   `credits` (GetBalance, GrantCreditsManually, ExpireCredits) ·
   `booking` (ListAvailableSlots, BookLesson, CancelBooking, ListMyBookings) ·
   `teacher` (SetAvailability, AddException, ListBookings, MarkAttendance, UpsertProduct, ListOrders) ·
   `leads` (CaptureLead, ConfirmLead, ExportLeads) ·
   `jobs` (SendBookingReminders, ExpireStaleOrders)
3. `app/fakes`: hand-written in-memory implementations of every port. No mocking library.

## Critical semantics

- `HandlePaymentWebhook` is **idempotent by payment event id**. Second delivery of the same event changes
  nothing and returns nil.
- `BookLesson` runs inside `TxManager.InTx`: read balance → domain check → debit → insert booking. A failure
  anywhere leaves no partial state.
- `CancelBooking` asks the domain for the outcome, then applies it - the use case never re-derives the policy.
- `IssueDownloadURL` checks the entitlement **before** asking `FileStorage` for a URL.

## Definition of done

- [ ] `make arch` passes - `app` imports no `database/sql`, no `net/http`, no `encoding/json`, no adapters.
- [ ] Use-case tests against fakes cover: repeated webhook grants exactly one entitlement and one credit batch ·
      booking with zero credits fails with `ErrInsufficientCredits` and writes nothing · booking debits exactly
      one credit · cancel at 25h refunds, at 23h consumes · download URL denied without entitlement · lead
      confirmation is idempotent.
- [ ] Zero use case reaches into another use case; shared behaviour lives in the domain.
