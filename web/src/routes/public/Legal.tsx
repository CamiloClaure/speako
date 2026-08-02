import type { ReactNode } from "react";

/**
 * Terms and privacy. These describe what the code actually does - the
 * cancellation window, the credit expiry, the seven-day withdrawal right and
 * the data actually collected. If the code changes, this page changes with it.
 */

function Page({ title, children }: { title: string; children: ReactNode }) {
  return (
    <article className="mx-auto flex w-full max-w-2xl flex-col gap-5 px-5 py-14 [&_h2]:mt-4 [&_h2]:font-display [&_h2]:text-xl [&_h2]:font-bold [&_li]:ml-5 [&_li]:list-disc [&_p]:text-muted [&_ul]:flex [&_ul]:flex-col [&_ul]:gap-2 [&_ul]:text-muted">
      <h1 className="text-3xl">{title}</h1>
      {children}
    </article>
  );
}

export function Termos() {
  return (
    <Page title="Termos de uso">
      <p>Última atualização: agosto de 2026.</p>

      <h2>O que a Speako oferece</h2>
      <p>
        Materiais digitais para download e aulas particulares de inglês por
        videochamada, vendidas em pacotes de créditos. As aulas acontecem em
        plataforma externa (Google Meet); o link é enviado com a confirmação do
        agendamento.
      </p>

      <h2>Créditos e validade</h2>
      <ul>
        <li>Um crédito equivale a uma aula particular de 60 minutos.</li>
        <li>Cada pacote tem prazo de validade contado a partir da confirmação do pagamento.</li>
        <li>Créditos vencidos não são restituídos nem prorrogados.</li>
        <li>O saldo e a data de vencimento ficam visíveis no seu painel.</li>
      </ul>

      <h2>Agendamento e cancelamento</h2>
      <ul>
        <li>O agendamento exige antecedência mínima de 12 horas.</li>
        <li>Cancelamento com 24 horas ou mais de antecedência devolve o crédito.</li>
        <li>Cancelamento com menos de 24 horas consome o crédito.</li>
        <li>Cancelamento pela professora sempre devolve o crédito, a qualquer momento.</li>
        <li>Ausência sem aviso consome o crédito.</li>
      </ul>

      <h2>Direito de arrependimento</h2>
      <p>
        Nos termos do art. 49 do Código de Defesa do Consumidor, você pode
        desistir da compra em até 7 dias corridos contados do pagamento,
        recebendo o valor de volta. Basta escrever para ola@speako.com.br.
        Créditos já utilizados nesse período são descontados do reembolso.
      </p>

      <h2>Pagamentos</h2>
      <p>
        Os pagamentos são processados por parceiro externo. A Speako não
        armazena dados de cartão de crédito em nenhum momento.
      </p>

      <h2>Uso dos materiais</h2>
      <p>
        Os materiais são licenciados para uso pessoal do comprador. A
        redistribuição, revenda ou publicação dos arquivos não é permitida.
      </p>
    </Page>
  );
}

export function Privacidade() {
  return (
    <Page title="Política de privacidade">
      <p>Última atualização: agosto de 2026.</p>

      <h2>Quais dados coletamos</h2>
      <ul>
        <li>Seu e-mail, para criar a conta e enviar o link de acesso.</li>
        <li>Seu nome e fuso horário, para exibir os horários corretamente.</li>
        <li>Histórico de compras, créditos e agendamentos.</li>
        <li>
          Ao se inscrever para receber material gratuito: a data, a hora e o
          endereço IP do consentimento, como exige a LGPD.
        </li>
      </ul>

      <h2>O que não coletamos</h2>
      <p>
        Não armazenamos senhas - o acesso é por link enviado ao seu e-mail. Não
        armazenamos dados de cartão. Não usamos cookies de rastreamento
        publicitário; o único cookie é o da sua sessão.
      </p>

      <h2>Como usamos</h2>
      <p>
        Para operar a plataforma: autenticar seu acesso, liberar o que você
        comprou, confirmar agendamentos e enviar lembretes de aula. E-mails de
        divulgação só são enviados a quem confirmou a inscrição.
      </p>

      <h2>Seus direitos</h2>
      <p>
        Você pode pedir acesso, correção ou exclusão dos seus dados a qualquer
        momento escrevendo para ola@speako.com.br. Excluímos ou anonimizamos os
        dados em até 30 dias, preservando apenas o que a legislação fiscal
        obriga a manter.
      </p>

      <h2>Cancelar e-mails</h2>
      <p>
        Todo e-mail de divulgação traz link de descadastro. E-mails
        transacionais (acesso, confirmação de compra e de aula) continuam sendo
        enviados enquanto sua conta existir.
      </p>
    </Page>
  );
}
